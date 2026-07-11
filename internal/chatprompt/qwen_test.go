package chatprompt

import (
	"strings"
	"testing"
)

func TestQwenFormatterFormatsRoleContentAndBoundaries(t *testing.T) {
	got, err := (QwenFormatter{}).FormatMessage(Message{
		Role:    "user",
		Content: "你好",
	})
	if err != nil {
		t.Fatalf("FormatMessage() error = %v", err)
	}

	want := "<|im_start|>user\n你好<|im_end|>\n"
	if got != want {
		t.Fatalf("FormatMessage() = %q, want %q", got, want)
	}
}

func TestQwenFormatterSupportsChatRoles(t *testing.T) {
	formatter := QwenFormatter{}
	for _, role := range []string{"system", "user", "assistant", "tool"} {
		t.Run(role, func(t *testing.T) {
			got, err := formatter.FormatMessage(Message{Role: role, Content: "x"})
			if err != nil {
				t.Fatalf("FormatMessage() error = %v", err)
			}
			if !strings.Contains(got, "<|im_start|>"+role+"\n") {
				t.Fatalf("FormatMessage() = %q, want role marker", got)
			}
		})
	}
}

func TestQwenFormatterRejectsUnknownRole(t *testing.T) {
	_, err := (QwenFormatter{}).FormatMessage(Message{Role: "unknown", Content: "x"})
	if err == nil || !strings.Contains(err.Error(), "unsupported message role") {
		t.Fatalf("FormatMessage() error = %v, want unsupported role error", err)
	}
}

func TestQwenFormatterReturnsAssistantPrefix(t *testing.T) {
	got := (QwenFormatter{}).AssistantPrefix()
	want := "<|im_start|>assistant\n"
	if got != want {
		t.Fatalf("AssistantPrefix() = %q, want %q", got, want)
	}
}

func TestQwenFormatterRendersConversationAndAssistantPrefix(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "规则"},
		{Role: "user", Content: "旧问题"},
		{Role: "assistant", Content: "旧回答"},
		{Role: "user", Content: "新问题"},
	}

	got, err := (QwenFormatter{}).Render(messages, true)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "<|im_start|>system\n规则<|im_end|>\n" +
		"<|im_start|>user\n旧问题<|im_end|>\n" +
		"<|im_start|>assistant\n旧回答<|im_end|>\n" +
		"<|im_start|>user\n新问题<|im_end|>\n" +
		"<|im_start|>assistant\n"
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestQwenFormatterRenderRejectsInvalidMessage(t *testing.T) {
	_, err := (QwenFormatter{}).Render([]Message{{Role: "unknown", Content: "x"}}, true)
	if err == nil || !strings.Contains(err.Error(), "message 0") {
		t.Fatalf("Render() error = %v, want indexed message error", err)
	}
}
