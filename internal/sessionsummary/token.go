package sessionsummary

import (
	"fmt"

	"offline-rag-go-lab/internal/chatprompt"
)

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type FormattedMessageCounter struct {
	counter   TextTokenCounter
	formatter chatprompt.QwenFormatter
}

func NewFormattedMessageCounter(counter TextTokenCounter, formatter chatprompt.QwenFormatter) FormattedMessageCounter {
	return FormattedMessageCounter{counter: counter, formatter: formatter}
}

func (c FormattedMessageCounter) CountMessage(message SourceMessage) (int, error) {
	if c.counter == nil {
		return 0, fmt.Errorf("text token counter is required")
	}
	formatted, err := c.formatter.FormatMessage(chatprompt.Message{
		Role:    message.Role,
		Content: message.Content,
	})
	if err != nil {
		return 0, fmt.Errorf("format message %d: %w", message.ID, err)
	}
	count, _, _, err := c.counter.CountText(formatted)
	if err != nil {
		return 0, fmt.Errorf("count message %d tokens: %w", message.ID, err)
	}
	return count, nil
}
