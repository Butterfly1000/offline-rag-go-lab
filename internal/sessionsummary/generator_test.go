package sessionsummary

import (
	"errors"
	"strings"
	"testing"
)

type fakeSummaryTextGenerator struct {
	result    string
	err       error
	model     string
	system    string
	prompt    string
	maxTokens int
}

func (g *fakeSummaryTextGenerator) GenerateText(model, system, prompt string, maxTokens int) (string, error) {
	g.model = model
	g.system = system
	g.prompt = prompt
	g.maxTokens = maxTokens
	return g.result, g.err
}

func TestGeneratorUpdatesSummaryAndTrimsOutput(t *testing.T) {
	client := &fakeSummaryTextGenerator{result: "<updated_summary>\n  新摘要  \n</updated_summary>"}
	generator := NewGenerator(client)

	got, err := generator.Update("qwen:7b", "旧摘要", []SourceMessage{{ID: 21, Role: "user", Content: "新事实"}}, 512)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got != "新摘要" {
		t.Fatalf("Update() = %q, want trimmed summary", got)
	}
	if client.model != "qwen:7b" || client.maxTokens != 512 {
		t.Fatalf("GenerateText() model=%q max=%d", client.model, client.maxTokens)
	}
	if client.system != SummarySystemPrompt || !strings.Contains(client.prompt, "旧摘要") {
		t.Fatalf("GenerateText() system=%q prompt=%q", client.system, client.prompt)
	}
}

func TestGeneratorValidatesConfigurationAndInput(t *testing.T) {
	messages := []SourceMessage{{ID: 21, Role: "user", Content: "x"}}
	if _, err := NewGenerator(nil).Update("qwen:7b", "", messages, 512); err == nil {
		t.Fatal("Update() error = nil, want client error")
	}
	if _, err := NewGenerator(&fakeSummaryTextGenerator{}).Update("", "", messages, 512); err == nil {
		t.Fatal("Update() error = nil, want model error")
	}
	if _, err := NewGenerator(&fakeSummaryTextGenerator{}).Update("qwen:7b", "", messages, 0); err == nil {
		t.Fatal("Update() error = nil, want max tokens error")
	}
}

func TestGeneratorPropagatesClientErrorAndRejectsEmptyOutput(t *testing.T) {
	messages := []SourceMessage{{ID: 21, Role: "user", Content: "x"}}
	_, err := NewGenerator(&fakeSummaryTextGenerator{err: errors.New("ollama failed")}).Update("qwen:7b", "", messages, 512)
	if err == nil || !strings.Contains(err.Error(), "generate summary") {
		t.Fatalf("Update() error = %v, want client context", err)
	}
	_, err = NewGenerator(&fakeSummaryTextGenerator{result: "  \n"}).Update("qwen:7b", "", messages, 512)
	if err == nil || !strings.Contains(err.Error(), "empty summary") {
		t.Fatalf("Update() error = %v, want empty output error", err)
	}
}
