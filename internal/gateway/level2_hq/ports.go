// Package hq（总指挥部）定义系统边界接口与主编排 App。
// ports.go 只声明「插槽」——各能力可替换，便于测试注入 mock 或未来接真实 Qdrant/Ollama。
package hq

import world "offline-rag-go-lab/internal/gateway/level1_world"

// KnowledgeStore 知识库抽象：写入 chunk、按问题搜索。
// 默认实现是 level3_store.MemoryKnowledgeStore（内存 slice 模拟 Qdrant）。
type KnowledgeStore interface {
	Upsert(chunks []world.KnowledgeChunk)                                      // 插入或按 ChunkID 覆盖更新
	Search(question string, topK int, threshold float64) []world.RetrievalHit // 相似度搜索，返回 topK 且 score >= threshold
}

// Retriever 检索服务抽象：把用户问题转成 RetrievalResult。
// 默认实现是 level4_retrieval.RetrievalService。
type Retriever interface {
	Retrieve(question string) world.RetrievalResult
}

// PromptBuilder 把命中 chunk 和用户问题拼成发给 LLM 的 prompt 字符串。
// 默认实现是 boss.StaticPromptBuilder。
type PromptBuilder interface {
	Build(question string, hits []world.RetrievalHit, maxChars int) string
}

// AnswerGenerator 根据问题和命中 chunk 生成最终回答。
// 默认实现是 boss.MockAnswerGenerator（拼接知识片段，非真实 LLM）。
type AnswerGenerator interface {
	Generate(question string, hits []world.RetrievalHit, maxChars int) string
}

// Compressor 对检索命中做去重、限量、截断，控制进入 prompt 的体量。
// 默认实现是 level6_compression.SimpleCompressor。
type Compressor interface {
	Compress(hits []world.RetrievalHit, maxChunks int, maxChars int) []world.RetrievalHit
}

// ConversationLogger 每次 Chat 成功后追加一条日志。
// 默认实现是 boss.JSONLConversationLogger（按天一个 .jsonl 文件）。
type ConversationLogger interface {
	AppendLog(req world.ChatRequest, resp world.ChatResponse, hits []world.RetrievalHit) error
}

// AppDeps 是依赖注入容器：字段为 nil 时 NewAppWithDeps 使用对应默认 mock 实现。
// 测试或接真实组件时，只替换需要 mock 的字段即可。
type AppDeps struct {
	Store         KnowledgeStore     // 知识存储
	Retriever     Retriever          // 检索
	PromptBuilder PromptBuilder      // prompt 组装（debug/prompt 用）
	Generator     AnswerGenerator    // 回答生成
	Compressor    Compressor         // 命中压缩
	Logger        ConversationLogger // 对话日志
}
