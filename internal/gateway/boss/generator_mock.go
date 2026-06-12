package boss

import (
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

type MockAnswerGenerator struct{}

func (g MockAnswerGenerator) Generate(question string, hits []world.RetrievalHit, maxChars int) string {
	if len(hits) == 0 {
		return "当前未命中知识库，我只能基于通用能力回答：" + question
	}

	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		parts = append(parts, shared.Truncate(hit.Text, maxChars))
	}
	return "根据知识库，" + strings.Join(parts, " ")
}
