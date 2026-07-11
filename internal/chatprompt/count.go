package chatprompt

import "fmt"

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type TokenUsage struct {
	Rendered    string
	TotalTokens int
}

type TokenCounter struct {
	counter   TextTokenCounter
	formatter QwenFormatter
}

func NewTokenCounter(counter TextTokenCounter, formatter QwenFormatter) TokenCounter {
	return TokenCounter{counter: counter, formatter: formatter}
}

// Count renders the complete conversation before tokenization. This avoids
// treating separately counted message bodies as the final prompt size.
func (c TokenCounter) Count(messages []Message, includeAssistantPrefix bool) (TokenUsage, error) {
	rendered, err := c.formatter.Render(messages, includeAssistantPrefix)
	if err != nil {
		return TokenUsage{}, fmt.Errorf("render conversation: %w", err)
	}

	total, _, _, err := c.counter.CountText(rendered)
	if err != nil {
		return TokenUsage{}, fmt.Errorf("count rendered conversation tokens: %w", err)
	}
	return TokenUsage{Rendered: rendered, TotalTokens: total}, nil
}
