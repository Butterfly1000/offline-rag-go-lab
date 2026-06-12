package hq

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	boss "offline-rag-go-lab/internal/gateway/boss"
	world "offline-rag-go-lab/internal/gateway/level1_world"
	store "offline-rag-go-lab/internal/gateway/level3_store"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
	chunking "offline-rag-go-lab/internal/gateway/level5_chunking"
	compression "offline-rag-go-lab/internal/gateway/level6_compression"
	"offline-rag-go-lab/internal/gateway/shared"
)

type App struct {
	config        world.Config
	store         KnowledgeStore
	retriever     Retriever
	compressor    Compressor
	promptBuilder PromptBuilder
	generator     AnswerGenerator
	logger        ConversationLogger
}

func NewApp(cfg world.Config) *App {
	return NewAppWithDeps(cfg, AppDeps{})
}

func NewAppWithDeps(cfg world.Config, deps AppDeps) *App {
	shared.MustMkdirAll(cfg.LogDir)
	shared.MustMkdirAll(cfg.DocDir)

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

func (a *App) SplitPreview(req world.IngestRequest) (world.SplitPreviewResponse, error) {
	chunks, err := chunking.BuildChunks(req)
	if err != nil {
		return world.SplitPreviewResponse{}, err
	}

	resp := world.SplitPreviewResponse{
		DocumentID: req.DocumentID,
		Chunks:     make([]world.SplitPreviewItem, 0, len(chunks)),
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

func (a *App) IngestText(req world.IngestRequest) (world.IngestResponse, error) {
	chunks, err := chunking.BuildChunks(req)
	if err != nil {
		return world.IngestResponse{}, err
	}

	a.store.Upsert(chunks)

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
			Score:      shared.Round4(hit.Score),
			Preview:    shared.Truncate(hit.Text, 120),
		})
	}
	return resp
}

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
		hits = a.retriever.Retrieve(req.Question).Hits
	}
	selected := a.compressor.Compress(hits, a.config.PromptMaxChunks, a.config.PromptMaxChars)

	resp := world.ChatResponse{
		Answer:          a.generator.Generate(req.Question, selected, a.config.PromptMaxChars),
		UsedKnowledge:   len(selected) > 0,
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
		return world.ChatResponse{}, err
	}
	return resp, nil
}
