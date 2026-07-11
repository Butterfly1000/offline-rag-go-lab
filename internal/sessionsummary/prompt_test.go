package sessionsummary

import (
	"strings"
	"testing"
)

func TestBuildUpdatePromptSeparatesAndEscapesInputs(t *testing.T) {
	prompt, err := BuildUpdatePrompt("偏好 <Go> & XML", []SourceMessage{
		{ID: 21, Role: "user", Content: "使用 <tag>"},
		{ID: 22, Role: "assistant", Content: "已确认 & 记录"},
	})
	if err != nil {
		t.Fatalf("BuildUpdatePrompt() error = %v", err)
	}
	for _, want := range []string{
		"<previous_summary>",
		"偏好 &lt;Go&gt; &amp; XML",
		"</previous_summary>",
		"<new_messages>",
		"[id=21 role=user] 使用 &lt;tag&gt;",
		"[id=22 role=assistant] 已确认 &amp; 记录",
		"</new_messages>",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("BuildUpdatePrompt() = %q, want %q", prompt, want)
		}
	}
	if strings.Index(prompt, "id=21") > strings.Index(prompt, "id=22") {
		t.Fatalf("BuildUpdatePrompt() changed message order: %q", prompt)
	}
}

func TestBuildUpdatePromptRejectsEmptyOrInvalidMessages(t *testing.T) {
	if _, err := BuildUpdatePrompt("old", nil); err == nil {
		t.Fatal("BuildUpdatePrompt() error = nil, want empty messages error")
	}
	if _, err := BuildUpdatePrompt("old", []SourceMessage{{ID: 2}, {ID: 1}}); err == nil {
		t.Fatal("BuildUpdatePrompt() error = nil, want order error")
	}
}
