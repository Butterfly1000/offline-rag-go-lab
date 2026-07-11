package chatprompt

import (
	"errors"
	"strings"
	"testing"
)

type recordingTextCounter struct {
	count int
	err   error
	calls []string
}

func (c *recordingTextCounter) CountText(text string) (int, []string, []int, error) {
	c.calls = append(c.calls, text)
	if c.err != nil {
		return 0, nil, nil, c.err
	}
	return c.count, []string{"token"}, []int{1}, nil
}

func TestTokenCounterCountsRenderedConversationOnce(t *testing.T) {
	raw := &recordingTextCounter{count: 17}
	counter := NewTokenCounter(raw, QwenFormatter{})

	usage, err := counter.Count([]Message{{Role: "user", Content: "问题"}}, true)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if usage.TotalTokens != 17 {
		t.Fatalf("Count() total = %d, want 17", usage.TotalTokens)
	}
	if len(raw.calls) != 1 {
		t.Fatalf("CountText() calls = %d, want 1", len(raw.calls))
	}
	if usage.Rendered != raw.calls[0] {
		t.Fatalf("Count() rendered = %q, CountText input = %q", usage.Rendered, raw.calls[0])
	}
	if !strings.HasSuffix(usage.Rendered, "<|im_start|>assistant\n") {
		t.Fatalf("Count() rendered = %q, want assistant prefix", usage.Rendered)
	}
}

func TestTokenCounterPropagatesFormatterError(t *testing.T) {
	raw := &recordingTextCounter{count: 17}
	counter := NewTokenCounter(raw, QwenFormatter{})

	_, err := counter.Count([]Message{{Role: "unknown", Content: "x"}}, true)
	if err == nil || !strings.Contains(err.Error(), "render conversation") {
		t.Fatalf("Count() error = %v, want render context", err)
	}
	if len(raw.calls) != 0 {
		t.Fatalf("CountText() calls = %d, want 0 after render failure", len(raw.calls))
	}
}

func TestTokenCounterPropagatesTokenizerError(t *testing.T) {
	raw := &recordingTextCounter{err: errors.New("tokenizer failed")}
	counter := NewTokenCounter(raw, QwenFormatter{})

	_, err := counter.Count([]Message{{Role: "user", Content: "x"}}, true)
	if err == nil || !strings.Contains(err.Error(), "count rendered conversation tokens") {
		t.Fatalf("Count() error = %v, want tokenizer context", err)
	}
}
