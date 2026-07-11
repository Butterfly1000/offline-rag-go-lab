package chatprompt

import (
	"fmt"
	"strings"
)

const (
	qwenMessageStart = "<|im_start|>"
	qwenMessageEnd   = "<|im_end|>"
)

type Message struct {
	Role    string
	Content string
}

type QwenFormatter struct{}

// FormatMessage includes the role and Qwen message boundaries because all of
// them consume context tokens; counting Content alone misses this overhead.
func (QwenFormatter) FormatMessage(message Message) (string, error) {
	if !isSupportedRole(message.Role) {
		return "", fmt.Errorf("unsupported message role %q", message.Role)
	}

	var formatted strings.Builder
	formatted.WriteString(qwenMessageStart)
	formatted.WriteString(message.Role)
	formatted.WriteByte('\n')
	formatted.WriteString(message.Content)
	formatted.WriteString(qwenMessageEnd)
	formatted.WriteByte('\n')
	return formatted.String(), nil
}

func (QwenFormatter) AssistantPrefix() string {
	return qwenMessageStart + "assistant\n"
}

// Render preserves message order and optionally appends the unfinished
// assistant prefix that tells the model where generated content begins.
func (f QwenFormatter) Render(messages []Message, includeAssistantPrefix bool) (string, error) {
	var rendered strings.Builder
	for i, message := range messages {
		formatted, err := f.FormatMessage(message)
		if err != nil {
			return "", fmt.Errorf("format message %d: %w", i, err)
		}
		rendered.WriteString(formatted)
	}
	if includeAssistantPrefix {
		rendered.WriteString(f.AssistantPrefix())
	}
	return rendered.String(), nil
}

func isSupportedRole(role string) bool {
	switch role {
	case "system", "user", "assistant", "tool":
		return true
	default:
		return false
	}
}
