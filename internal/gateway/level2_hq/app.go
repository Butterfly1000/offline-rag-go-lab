package hq

import (
	"errors"     // Chat 校验失败时返回 errors.New
	"os"         // IngestText 用 os.WriteFile 落盘原文
	"path/filepath" // 拼接文档保存路径
	"strings"    // TrimSpace 校验必填字段
	"time"       // Chat 计时 LatencyMS

	boss "offline-rag-go-lab/internal/gateway/boss"
	world "offline-rag-go-lab/internal/gateway/level1_world"
	store "offline-rag-go-lab/internal/gateway/level3_store"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
	chunking "offline-rag-go-lab/internal/gateway/level5_chunking"
	compression "offline-rag-go-lab/internal/gateway/level6_compression"
	"offline-rag-go-lab/internal/gateway/shared"
)

// App 是 RAG 主编排器：持有配置与各接口实现，对外提供 SplitPreview / Ingest / Debug / Chat 方法。
type App struct {
	config        world.Config         // 全局配置副本
	store         KnowledgeStore       // 知识库
	retriever     Retriever            // 检索
	compressor    Compressor           // 压缩
	promptBuilder PromptBuilder        // prompt 组装
	generator     AnswerGenerator      // 回答生成
	logger        ConversationLogger   // 日志
}

// NewApp 使用全部默认依赖创建 App（生产入口常用）。
func NewApp(cfg world.Config) *App {
	return NewAppWithDeps(cfg, AppDeps{}) // 空 deps → 各组件走默认 mock
}

// NewAppWithDeps 创建 App，deps 里非 nil 的字段会覆盖默认实现（测试/扩展用）。
func NewAppWithDeps(cfg world.Config, deps AppDeps) *App {
	// 启动时确保日志目录和文档目录存在；失败会 panic
	shared.MustMkdirAll(cfg.LogDir)
	shared.MustMkdirAll(cfg.DocDir)

	// 以下为配置兜底默认值，避免零值导致检索/ prompt 行为异常
	if cfg.RetrievalTopK <= 0 {
		cfg.RetrievalTopK = 5
	}
	if cfg.PromptMaxChunks <= 0 {
		cfg.PromptMaxChunks = 4
	}
	if cfg.PromptMaxChars <= 0 {
		cfg.PromptMaxChars = 1200
	}
	if cfg.ChatModel == "" {
		cfg.ChatModel = "mock-chat"
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "mock-embedding"
	}
	if cfg.KnowledgeCollection == "" {
		cfg.KnowledgeCollection = "knowledge_chunks"
	}

	// 依赖注入：nil 则用默认内存 store
	storeImpl := deps.Store
	if storeImpl == nil {
		storeImpl = store.NewMemoryKnowledgeStore()
	}

	retriever := deps.Retriever
	if retriever == nil {
		retriever = retrieval.NewRetrievalService(storeImpl, cfg.RetrievalTopK, cfg.ScoreThreshold)
	}

	promptBuilder := deps.PromptBuilder
	if promptBuilder == nil {
		promptBuilder = boss.StaticPromptBuilder{}
	}

	compressor := deps.Compressor
	if compressor == nil {
		compressor = compression.SimpleCompressor{}
	}

	generator := deps.Generator
	if generator == nil {
		generator = boss.MockAnswerGenerator{}
	}

	logger := deps.Logger
	if logger == nil {
		logger = boss.NewJSONLConversationLogger(cfg.LogDir, cfg.ChatModel, cfg.EmbeddingModel)
	}

	return &App{
		config:        cfg,
		store:         storeImpl,
		retriever:     retriever,
		compressor:    compressor,
		promptBuilder: promptBuilder,
		generator:     generator,
		logger:        logger,
	}
}

// SplitPreview 只切分文档、不入库，对应 POST /debug/split。
func (a *App) SplitPreview(req world.IngestRequest) (world.SplitPreviewResponse, error) {
	chunks, err := chunking.BuildChunks(req) // 调用 level5 切块
	if err != nil {
		return world.SplitPreviewResponse{}, err // 零值 + error 是 Go 常见错误返回模式
	}

	resp := world.SplitPreviewResponse{
		DocumentID: req.DocumentID,
		Chunks:     make([]world.SplitPreviewItem, 0, len(chunks)), // 预分配，避免多次扩容
	}
	for _, chunk := range chunks {
		resp.Chunks = append(resp.Chunks, world.SplitPreviewItem{
			ChunkID:    chunk.ChunkID,
			ChunkIndex: chunk.ChunkIndex,
			Text:       chunk.Text,
		})
	}
	return resp, nil
}

// IngestText 导入知识：切块 → 写入 store → 原文落盘，对应 POST /ingest。
func (a *App) IngestText(req world.IngestRequest) (world.IngestResponse, error) {
	chunks, err := chunking.BuildChunks(req)
	if err != nil {
		return world.IngestResponse{}, err
	}

	a.store.Upsert(chunks) // 写入内存知识库（同 ChunkID 会覆盖）

	// 原文保存为 DocDir/{document_id}.txt，0o644 = rw-r--r--
	docPath := filepath.Join(a.config.DocDir, req.DocumentID+".txt")
	if err := os.WriteFile(docPath, []byte(req.Text), 0o644); err != nil {
		return world.IngestResponse{}, err
	}

	return world.IngestResponse{
		DocumentID:     req.DocumentID,
		ChunkCount:     len(chunks),
		EmbeddingModel: a.config.EmbeddingModel,
		Status:         "ok",
	}, nil
}

// DebugRetrieval 返回检索中间结果，对应 GET /debug/retrieval。
func (a *App) DebugRetrieval(question string) world.DebugRetrievalResponse {
	result := a.retriever.Retrieve(question)

	resp := world.DebugRetrievalResponse{
		Question:           result.Question,
		NormalizedQuestion: result.NormalizedQuestion,
		Hits:               make([]world.DebugHit, 0, len(result.Hits)),
	}
	for _, hit := range result.Hits {
		resp.Hits = append(resp.Hits, world.DebugHit{
			DocumentID: hit.DocumentID,
			ChunkID:    hit.ChunkID,
			Title:      hit.Title,
			SourceRef:  hit.SourceRef,
			Score:      shared.Round4(hit.Score),       // 分数保留 4 位
			Preview:    shared.Truncate(hit.Text, 120), // 预览截断 120 字节
		})
	}
	return resp
}

// PromptPreview 走检索 → 压缩 → 拼 prompt，对应 GET /debug/prompt。
func (a *App) PromptPreview(question string) world.PromptPreviewResponse {
	result := a.retriever.Retrieve(question)
	selected := a.compressor.Compress(result.Hits, a.config.PromptMaxChunks, a.config.PromptMaxChars)

	resp := world.PromptPreviewResponse{
		Question:       question,
		SelectedChunks: make([]world.RetrievedChunk, 0, len(selected)),
		Prompt:         a.promptBuilder.Build(question, selected, a.config.PromptMaxChars),
	}
	for _, hit := range selected {
		resp.SelectedChunks = append(resp.SelectedChunks, world.RetrievedChunk{
			DocumentID: hit.DocumentID,
			ChunkID:    hit.ChunkID,
			Title:      hit.Title,
			SourceRef:  hit.SourceRef,
			Score:      shared.Round4(hit.Score),
		})
	}
	return resp
}

// Chat 完整问答链路：校验 → 可选检索 → 压缩 → 生成 → 写日志，对应 POST /chat。
func (a *App) Chat(req world.ChatRequest) (world.ChatResponse, error) {
	if strings.TrimSpace(req.SessionID) == "" {
		return world.ChatResponse{}, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return world.ChatResponse{}, errors.New("user_id is required")
	}
	if strings.TrimSpace(req.Question) == "" {
		return world.ChatResponse{}, errors.New("question is required")
	}

	start := time.Now()
	hits := []world.RetrievalHit{}
	if req.UseKnowledge {
		hits = a.retriever.Retrieve(req.Question).Hits // 只取 Hits，不关心 NormalizedQuestion
	}
	selected := a.compressor.Compress(hits, a.config.PromptMaxChunks, a.config.PromptMaxChars)

	resp := world.ChatResponse{
		Answer:          a.generator.Generate(req.Question, selected, a.config.PromptMaxChars),
		UsedKnowledge:   len(selected) > 0, // 有 chunk 进入上下文即为 true
		RetrievedChunks: make([]world.RetrievedChunk, 0, len(selected)),
		LatencyMS:       time.Since(start).Milliseconds(),
	}
	for _, hit := range selected {
		resp.RetrievedChunks = append(resp.RetrievedChunks, world.RetrievedChunk{
			DocumentID: hit.DocumentID,
			ChunkID:    hit.ChunkID,
			Title:      hit.Title,
			SourceRef:  hit.SourceRef,
			Score:      shared.Round4(hit.Score),
		})
	}

	if err := a.logger.AppendLog(req, resp, selected); err != nil {
		return world.ChatResponse{}, err // 写日志失败则整个 Chat 视为失败
	}
	return resp, nil
}
