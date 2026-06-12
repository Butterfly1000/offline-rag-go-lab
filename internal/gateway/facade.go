package gateway

import (
	world "offline-rag-go-lab/internal/gateway/level1_world"
	hq "offline-rag-go-lab/internal/gateway/level2_hq"
)

type App = hq.App
type AppDeps = hq.AppDeps

type Config = world.Config
type IngestRequest = world.IngestRequest
type IngestResponse = world.IngestResponse
type ChatRequest = world.ChatRequest
type ChatResponse = world.ChatResponse
type RetrievedChunk = world.RetrievedChunk
type DebugRetrievalResponse = world.DebugRetrievalResponse
type DebugHit = world.DebugHit
type SplitPreviewResponse = world.SplitPreviewResponse
type SplitPreviewItem = world.SplitPreviewItem
type PromptPreviewResponse = world.PromptPreviewResponse

type KnowledgeStore = hq.KnowledgeStore
type Retriever = hq.Retriever
type PromptBuilder = hq.PromptBuilder
type AnswerGenerator = hq.AnswerGenerator
type Compressor = hq.Compressor
type ConversationLogger = hq.ConversationLogger

func NewApp(cfg Config) *App {
	return hq.NewApp(cfg)
}

func NewAppWithDeps(cfg Config, deps AppDeps) *App {
	return hq.NewAppWithDeps(cfg, deps)
}
