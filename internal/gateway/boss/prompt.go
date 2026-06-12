package boss

import (
	"fmt"
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

type StaticPromptBuilder struct{}

func (b StaticPromptBuilder) Build(question string, hits []world.RetrievalHit, maxChars int) string {
	return BuildPrompt(question, hits, maxChars)
}

func BuildPrompt(question string, hits []world.RetrievalHit, maxChars int) string {
	var b strings.Builder
	b.WriteString("[System Instruction]\n")
	b.WriteString("你是一个离线知识问答助手。优先依据提供的上下文回答；如果上下文不足，请明确说明未命中知识库，不要编造。\n\n")
	b.WriteString("[Relevant Knowledge]\n")

	if len(hits) == 0 {
		b.WriteString("无命中知识。\n\n")
	} else {
		for i, hit := range hits {
			b.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, hit.Title, shared.Truncate(hit.Text, maxChars)))
		}
		b.WriteString("\n")
	}

	b.WriteString("[User Question]\n")
	b.WriteString(question)
	return b.String()
}
