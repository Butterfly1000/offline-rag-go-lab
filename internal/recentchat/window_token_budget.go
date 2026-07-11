package recentchat

import (
	"fmt"

	"offline-rag-go-lab/internal/chatprompt"
)

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type TokenBudgetWindowBuilder struct {
	counter   TextTokenCounter
	formatter *chatprompt.QwenFormatter
	strict    bool
}

func NewTokenBudgetWindowBuilder(counter TextTokenCounter) TokenBudgetWindowBuilder {
	return TokenBudgetWindowBuilder{counter: counter}
}

func NewFormattedTokenBudgetWindowBuilder(counter TextTokenCounter, formatter chatprompt.QwenFormatter) TokenBudgetWindowBuilder {
	return TokenBudgetWindowBuilder{
		counter:   counter,
		formatter: &formatter,
		strict:    true,
	}
}

func (b TokenBudgetWindowBuilder) Build(messages []Message, budget int) ([]Message, int, error) {
	if len(messages) == 0 {
		return messages, 0, nil
	}
	if budget <= 0 {
		if b.strict {
			return messages[:0], 0, nil
		}
		return messages, 0, nil
	}

	selected := make([]Message, 0, len(messages))
	used := 0

	for i := len(messages) - 1; i >= 0; i-- {
		text, err := b.textForCount(messages[i])
		if err != nil {
			return nil, 0, err
		}
		count, _, _, err := b.counter.CountText(text)
		if err != nil {
			return nil, 0, fmt.Errorf("count recent message %d tokens: %w", i, err)
		}

		if used+count > budget {
			if len(selected) == 0 && !b.strict {
				selected = append(selected, messages[i])
				used += count
			}
			break
		}

		selected = append(selected, messages[i])
		used += count
	}

	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}

	return selected, used, nil
}

func (b TokenBudgetWindowBuilder) textForCount(message Message) (string, error) {
	if b.formatter == nil {
		return message.Content, nil
	}

	formatted, err := b.formatter.FormatMessage(chatprompt.Message{
		Role:    string(message.Role),
		Content: message.Content,
	})
	if err != nil {
		return "", fmt.Errorf("format recent message: %w", err)
	}
	return formatted, nil
}
