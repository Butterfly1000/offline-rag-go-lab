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
	// len(hits) == 0 表示这次没有任何知识片段进入生成阶段。
	if len(hits) == 0 {
		// 没命中知识库时，直接返回一个兜底回答。
		return "当前未命中知识库，我只能基于通用能力回答：" + question
	}

	// make([]string, 0, len(hits))：
	// 创建一个字符串切片，用来收集每个命中 chunk 的文本。
	parts := make([]string, 0, len(hits))

	for _, hit := range hits {
		// shared.Truncate 是项目自己的工具函数，用来把文本截断到最多 maxChars 个字符。
		parts = append(parts, shared.Truncate(hit.Text, maxChars))
	}

	// strings.Join(parts, " ")：
	// 用空格把多个字符串拼接成一个字符串。
	return "根据知识库，" + strings.Join(parts, " ") // 简单空格拼接各 chunk 正文
}
