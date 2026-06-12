// Package gateway 是对外统一入口：类型别名 + 构造函数转发到 level2_hq。
// cmd/rag-gateway 只 import 这一层，避免调用方感知内部分包结构。
package gateway

import (
	world "offline-rag-go-lab/internal/gateway/level1_world"
	hq "offline-rag-go-lab/internal/gateway/level2_hq"
)

// 以下 type X = Y 是类型别名：对外名字在 gateway 包，底层仍是 hq/world 的定义。
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

// 接口类型也别名导出，便于测试里自定义 mock 并注入 NewAppWithDeps。
type KnowledgeStore = hq.KnowledgeStore
type Retriever = hq.Retriever
type PromptBuilder = hq.PromptBuilder
type AnswerGenerator = hq.AnswerGenerator
type Compressor = hq.Compressor
type ConversationLogger = hq.ConversationLogger

// NewApp 创建使用默认 mock 依赖的 App。
func NewApp(cfg Config) *App {
	return hq.NewApp(cfg)
}

// NewAppWithDeps 创建 App，可注入自定义 Store/Retriever 等。
func NewAppWithDeps(cfg Config, deps AppDeps) *App {
	return hq.NewAppWithDeps(cfg, deps)
}
