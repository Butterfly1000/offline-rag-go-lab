package recentchat

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type TokenBudgetWindowBuilder struct {
	counter TextTokenCounter
}

func NewTokenBudgetWindowBuilder(counter TextTokenCounter) TokenBudgetWindowBuilder {
	return TokenBudgetWindowBuilder{counter: counter}
}

func (b TokenBudgetWindowBuilder) Build(messages []Message, budget int) ([]Message, int, error) {
	if budget <= 0 || len(messages) == 0 {
		return messages, 0, nil
	}

	selected := make([]Message, 0, len(messages))
	used := 0

	for i := len(messages) - 1; i >= 0; i-- {
		count, _, _, err := b.counter.CountText(messages[i].Content)
		if err != nil {
			return nil, 0, err
		}

		if used+count > budget {
			if len(selected) == 0 {
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
