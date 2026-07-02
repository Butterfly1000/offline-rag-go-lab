package hq

import (
	"errors"        // Chat 校验失败时返回 errors.New
	"os"            // IngestText 用 os.WriteFile 落盘原文
	"path/filepath" // 拼接文档保存路径
	"strings"       // TrimSpace 校验必填字段
	"time"          // Chat 计时 LatencyMS

	boss "offline-rag-go-lab/internal/gateway/boss"
	world "offline-rag-go-lab/internal/gateway/level1_world"
	store "offline-rag-go-lab/internal/gateway/level3_store"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
	chunking "offline-rag-go-lab/internal/gateway/level5_chunking"
	compression "offline-rag-go-lab/internal/gateway/level6_compression"
	"offline-rag-go-lab/internal/gateway/shared"
)

// App 是 RAG 主编排器：持有配置与各接口实现，对外提供 SplitPreview / Ingest / Debug / Chat 方法。
// 重点不是“自己实现所有能力”，而是“把不同能力组织起来”。
type App struct {
	config        world.Config       // 全局配置，比如 topK、日志目录、prompt 限制
	store         KnowledgeStore     // 知识库存储：负责存和搜知识
	retriever     Retriever          // 检索器：负责根据问题找相关 chunk
	compressor    Compressor         // 压缩器：负责把命中的 chunk 做筛选、限量、裁剪
	promptBuilder PromptBuilder      // prompt 构造器：负责把问题和知识拼成 prompt
	generator     AnswerGenerator    // 回答生成器：负责产出最终 answer
	logger        ConversationLogger // 日志器：负责记录这次对话
}

// NewApp 使用全部默认依赖创建 App（生产入口常用）。
func NewApp(cfg world.Config) *App {
	return NewAppWithDeps(cfg, AppDeps{}) // 空 deps → 各组件走默认 mock
}

// NewAppWithDeps 创建 App，deps 里非 nil 的字段会覆盖默认实现（测试/扩展用）。
func NewAppWithDeps(cfg world.Config, deps AppDeps) *App {
	// 启动时确保日志目录和文档目录存在；失败会 panic
	// shared.MustMkdirAll 内部会调用 os.MkdirAll；
	// MkdirAll 的作用是：目录不存在就递归创建，存在就直接通过。
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
	// 如果调用方没有传入自定义 store，就使用默认的内存版知识库。
	storeImpl := deps.Store
	if storeImpl == nil {
		storeImpl = store.NewMemoryKnowledgeStore()
	}

	// 如果没有传入自定义检索器，就用默认教学版检索器。
	retriever := deps.Retriever
	if retriever == nil {
		retriever = retrieval.NewRetrievalService(storeImpl, cfg.RetrievalTopK, cfg.ScoreThreshold)
	}

	// 如果没有传入自定义 prompt 构造器，就用默认静态 prompt 构造器。
	promptBuilder := deps.PromptBuilder
	if promptBuilder == nil {
		promptBuilder = boss.StaticPromptBuilder{}
	}

	// 如果没有传入自定义压缩器，就用默认简单压缩器。
	compressor := deps.Compressor
	if compressor == nil {
		compressor = compression.SimpleCompressor{}
	}

	// 如果没有传入自定义回答生成器，就用默认 mock 回答生成器。
	generator := deps.Generator
	if generator == nil {
		generator = boss.MockAnswerGenerator{}
	}

	// 如果没有传入自定义日志器，就用默认 JSONL 日志器。
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
	// strings.TrimSpace 会去掉字符串首尾的空格、换行、制表符。
	// 这里的目的不是“美化文本”，而是防止用户传入 "   " 这种看起来非空、其实无意义的值。
	if strings.TrimSpace(req.SessionID) == "" {
		return world.ChatResponse{}, errors.New("session_id is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return world.ChatResponse{}, errors.New("user_id is required")
	}
	if strings.TrimSpace(req.Question) == "" {
		return world.ChatResponse{}, errors.New("question is required")
	}

	// time.Now() 取当前时间点。
	// 先记住开始时间，后面用来计算整次 chat 花了多久。
	start := time.Now()

	// 先准备一个空的命中结果切片。
	// []world.RetrievalHit{} 表示“当前有一个空列表”，不是 nil。
	hits := []world.RetrievalHit{}

	// 只有当 UseKnowledge = true 时，才真的去知识库检索。
	if req.UseKnowledge {
		hits = a.retriever.Retrieve(req.Question).Hits // 只取 Hits，不关心 NormalizedQuestion
	}

	// 检索结果不直接送去生成，而是先压缩。
	// 这样可以限制 chunk 数量和文本长度，避免上下文过大。
	selected := a.compressor.Compress(hits, a.config.PromptMaxChunks, a.config.PromptMaxChars)

	// 先构造响应骨架。
	resp := world.ChatResponse{
		// 生成器负责最终 answer。
		Answer: a.generator.Generate(req.Question, selected, a.config.PromptMaxChars),

		// len(selected) 是切片长度。
		// 只要最终进入生成阶段的 chunk 数量 > 0，就说明这次“用到了知识”。
		UsedKnowledge: len(selected) > 0, // 有 chunk 进入上下文即为 true

		// make([]T, 0, n)：
		// 创建一个长度为 0、容量为 n 的切片。
		// 好处是后面 append 时，通常不需要频繁扩容。
		RetrievedChunks: make([]world.RetrievedChunk, 0, len(selected)),

		// time.Since(start) 会得到“从 start 到现在”经过了多久。
		// Milliseconds() 再把这个时长转成毫秒整数。
		LatencyMS: time.Since(start).Milliseconds(),
	}

	// 把内部命中结果，转换成对外返回的摘要结构。
	for _, hit := range selected {
		resp.RetrievedChunks = append(resp.RetrievedChunks, world.RetrievedChunk{
			DocumentID: hit.DocumentID,
			ChunkID:    hit.ChunkID,
			Title:      hit.Title,
			SourceRef:  hit.SourceRef,
			// Round4 是项目自己的工具函数，用来把分数保留 4 位小数。
			Score: shared.Round4(hit.Score),
		})
	}

	// 最后追加日志。
	// 如果日志写失败，这次 Chat 整体也算失败，直接返回 error。
	if err := a.logger.AppendLog(req, resp, selected); err != nil {
		return world.ChatResponse{}, err // 写日志失败则整个 Chat 视为失败
	}
	return resp, nil
}
