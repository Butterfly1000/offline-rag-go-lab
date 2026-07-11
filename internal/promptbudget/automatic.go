package promptbudget

import (
	"fmt"

	"offline-rag-go-lab/internal/chatprompt"
)

type ContextProvider interface {
	ContextLength(model string) (int, error)
}

type ConversationCounter interface {
	Count(messages []chatprompt.Message, includeAssistantPrefix bool) (chatprompt.TokenUsage, error)
}

type AutomaticPlan struct {
	BudgetPlan
	RenderedFixedPrompt string
}

type AutomaticPlanner struct {
	contextProvider ContextProvider
	counter         ConversationCounter
}

func NewAutomaticPlanner(contextProvider ContextProvider, counter ConversationCounter) AutomaticPlanner {
	return AutomaticPlanner{contextProvider: contextProvider, counter: counter}
}

// Plan reads the model-owned context limit and counts the complete fixed
// prompt before assigning the remaining capacity to recent history.
func (p AutomaticPlanner) Plan(model string, fixed []chatprompt.Message, outputReserve int) (AutomaticPlan, error) {
	contextLimit, err := p.contextProvider.ContextLength(model)
	if err != nil {
		return AutomaticPlan{}, fmt.Errorf("read model context length: %w", err)
	}

	usage, err := p.counter.Count(fixed, true)
	if err != nil {
		return AutomaticPlan{}, fmt.Errorf("count fixed prompt tokens: %w", err)
	}

	budget, err := Plan(contextLimit, usage.TotalTokens, outputReserve)
	if err != nil {
		return AutomaticPlan{}, err
	}
	return AutomaticPlan{
		BudgetPlan:          budget,
		RenderedFixedPrompt: usage.Rendered,
	}, nil
}
