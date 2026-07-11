package sessionsummary

import (
	"fmt"
	"html"
	"strings"
)

const SummarySystemPrompt = `你是会话滚动摘要器。只输出更新后的摘要正文，不要输出 XML 标签、标题、列表、建议或过程解释。保留后续仍有效的用户偏好、目标、约束、事实和已确认决定；合并旧摘要与新增消息，删除临时闲聊、重复信息和已经失效的过程细节；不要编造输入中没有的事实。`

func BuildUpdatePrompt(previous string, messages []SourceMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("new messages are required")
	}
	if err := validateSourceMessages(messages); err != nil {
		return "", err
	}

	if strings.TrimSpace(previous) == "" {
		previous = "(empty)"
	}
	var prompt strings.Builder
	prompt.WriteString("<previous_summary>\n")
	prompt.WriteString(html.EscapeString(previous))
	prompt.WriteString("\n</previous_summary>\n<new_messages>\n")
	for _, message := range messages {
		_, _ = fmt.Fprintf(
			&prompt,
			"[id=%d role=%s] %s\n",
			message.ID,
			html.EscapeString(message.Role),
			html.EscapeString(message.Content),
		)
	}
	prompt.WriteString("</new_messages>")
	return prompt.String(), nil
}
