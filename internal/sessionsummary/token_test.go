package sessionsummary

import (
	"errors"
	"strings"
	"testing"

	"offline-rag-go-lab/internal/chatprompt"
)

type recordingSummaryTextCounter struct {
	count int
	err   error
	text  string
}

func (c *recordingSummaryTextCounter) CountText(text string) (int, []string, []int, error) {
	c.text = text
	return c.count, nil, nil, c.err
}

func TestFormattedMessageCounterCountsCompleteQwenMessage(t *testing.T) {
	raw := &recordingSummaryTextCounter{count: 12}
	counter := NewFormattedMessageCounter(raw, chatprompt.QwenFormatter{})

	got, err := counter.CountMessage(SourceMessage{ID: 21, Role: "user", Content: "你好"})
	if err != nil {
		t.Fatalf("CountMessage() error = %v", err)
	}
	if got != 12 {
		t.Fatalf("CountMessage() = %d, want 12", got)
	}
	want := "<|im_start|>user\n你好<|im_end|>\n"
	if raw.text != want {
		t.Fatalf("CountText() input = %q, want %q", raw.text, want)
	}
}

func TestFormattedMessageCounterPropagatesFormattingError(t *testing.T) {
	counter := NewFormattedMessageCounter(&recordingSummaryTextCounter{}, chatprompt.QwenFormatter{})

	_, err := counter.CountMessage(SourceMessage{ID: 21, Role: "unknown", Content: "x"})
	if err == nil || !strings.Contains(err.Error(), "format message 21") {
		t.Fatalf("CountMessage() error = %v, want formatting context", err)
	}
}

func TestFormattedMessageCounterPropagatesTokenizerError(t *testing.T) {
	counter := NewFormattedMessageCounter(
		&recordingSummaryTextCounter{err: errors.New("tokenizer failed")},
		chatprompt.QwenFormatter{},
	)

	_, err := counter.CountMessage(SourceMessage{ID: 21, Role: "user", Content: "x"})
	if err == nil || !strings.Contains(err.Error(), "count message 21 tokens") {
		t.Fatalf("CountMessage() error = %v, want tokenizer context", err)
	}
}

func TestFormattedMessageCounterRequiresRawCounter(t *testing.T) {
	counter := NewFormattedMessageCounter(nil, chatprompt.QwenFormatter{})

	_, err := counter.CountMessage(SourceMessage{ID: 21, Role: "user", Content: "x"})
	if err == nil || !strings.Contains(err.Error(), "token counter is required") {
		t.Fatalf("CountMessage() error = %v, want missing counter error", err)
	}
}
