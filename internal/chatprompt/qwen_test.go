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
