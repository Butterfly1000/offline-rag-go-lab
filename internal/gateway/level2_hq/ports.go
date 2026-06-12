package hq

import world "offline-rag-go-lab/internal/gateway/level1_world"

type KnowledgeStore interface {
	Upsert(chunks []world.KnowledgeChunk)
	Search(question string, topK int, threshold float64) []world.RetrievalHit
}

type Retriever interface {
	Retrieve(question string) world.RetrievalResult
}

type PromptBuilder interface {
	Build(question string, hits []world.RetrievalHit, maxChars int) string
}

type AnswerGenerator interface {
	Generate(question string, hits []world.RetrievalHit, maxChars int) string
}

type Compressor interface {
	Compress(hits []world.RetrievalHit, maxChunks int, maxChars int) []world.RetrievalHit
}

type ConversationLogger interface {
	AppendLog(req world.ChatRequest, resp world.ChatResponse, hits []world.RetrievalHit) error
}

type AppDeps struct {
	Store         KnowledgeStore
	Retriever     Retriever
	PromptBuilder PromptBuilder
	Generator     AnswerGenerator
	Compressor    Compressor
	Logger        ConversationLogger
}
