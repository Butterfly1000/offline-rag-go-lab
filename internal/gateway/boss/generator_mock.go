package boss

import (
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

// MockAnswerGenerator 模拟 LLM：有命中则拼接知识片段，无命中则返回固定兜底话术。
type MockAnswerGenerator struct{}

// Generate 实现 hq.AnswerGenerator；不接真实 Ollama 时用于跑通闭环。
func (g MockAnswerGenerator) Generate(question string, hits []world.RetrievalHit, maxChars int) string {
	if len(hits) == 0 {
		return "当前未命中知识库，我只能基于通用能力回答：" + question
	}

	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		parts = append(parts, shared.Truncate(hit.Text, maxChars))
	}
	return "根据知识库，" + strings.Join(parts, " ") // 简单空格拼接各 chunk 正文
}
