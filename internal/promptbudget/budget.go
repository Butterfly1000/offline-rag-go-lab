package promptbudget

import "fmt"

type BudgetPlan struct {
	ContextLimit           int
	FixedInputTokens       int
	OutputReserve          int
	AvailableHistoryTokens int
}

// Plan reserves capacity for the already-rendered fixed prompt and future
// model output, then assigns the remaining context to recent history.
func Plan(contextLimit int, fixedInputTokens int, outputReserve int) (BudgetPlan, error) {
	if contextLimit <= 0 {
		return BudgetPlan{}, fmt.Errorf("context limit must be positive: %d", contextLimit)
	}
	if fixedInputTokens < 0 {
		return BudgetPlan{}, fmt.Errorf("fixed input tokens must not be negative: %d", fixedInputTokens)
	}
	if outputReserve < 0 {
		return BudgetPlan{}, fmt.Errorf("output reserve must not be negative: %d", outputReserve)
	}

	used := fixedInputTokens + outputReserve
	if used > contextLimit {
		return BudgetPlan{}, fmt.Errorf("fixed input and output reserve (%d) exceeds context limit (%d)", used, contextLimit)
	}

	return BudgetPlan{
		ContextLimit:           contextLimit,
		FixedInputTokens:       fixedInputTokens,
		OutputReserve:          outputReserve,
		AvailableHistoryTokens: contextLimit - used,
	}, nil
}
