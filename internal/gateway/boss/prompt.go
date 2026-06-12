// Package boss 是 RAG 链末端：prompt 组装、mock 回答生成、JSONL 日志（Boss 战三件套）。
package boss

import (
	"fmt"     // 格式化带序号的 knowledge 行
	"strings" // strings.Builder 高效拼接大字符串

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

// StaticPromptBuilder 实现 hq.PromptBuilder，委托给包级函数 BuildPrompt。
type StaticPromptBuilder struct{}

// Build 满足 PromptBuilder 接口。
func (b StaticPromptBuilder) Build(question string, hits []world.RetrievalHit, maxChars int) string {
	return BuildPrompt(question, hits, maxChars)
}

// BuildPrompt 拼出发给 LLM 的完整 prompt，分 System / Knowledge / Question 三段。
func BuildPrompt(question string, hits []world.RetrievalHit, maxChars int) string {
	var b strings.Builder // 比反复 + 拼接字符串更高效
	b.WriteString("[System Instruction]\n")
	b.WriteString("你是一个离线知识问答助手。优先依据提供的上下文回答；如果上下文不足，请明确说明未命中知识库，不要编造。\n\n")
	b.WriteString("[Relevant Knowledge]\n")

	if len(hits) == 0 {
		b.WriteString("无命中知识。\n\n")
	} else {
		for i, hit := range hits {
			// 每条：序号. [标题] 正文（截断）
			b.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, hit.Title, shared.Truncate(hit.Text, maxChars)))
		}
		b.WriteString("\n")
	}

	b.WriteString("[User Question]\n")
	b.WriteString(question)
	return b.String()
}
