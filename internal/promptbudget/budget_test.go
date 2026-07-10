package promptbudget

import (
	"strings"
	"testing"
)

func TestPlanReturnsAvailableHistoryBudget(t *testing.T) {
	plan, err := Plan(32768, 88, 2048)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := BudgetPlan{
		ContextLimit:           32768,
		FixedInputTokens:       88,
		OutputReserve:          2048,
		AvailableHistoryTokens: 30632,
	}
	if plan != want {
		t.Fatalf("Plan() = %+v, want %+v", plan, want)
	}
}

func TestPlanRejectsBudgetThatExceedsContextLimit(t *testing.T) {
	_, err := Plan(100, 80, 30)
	if err == nil {
		t.Fatal("Plan() error = nil, want over-budget error")
	}
	if !strings.Contains(err.Error(), "exceeds context limit") {
		t.Fatalf("Plan() error = %q, want context limit explanation", err)
	}
}

func TestPlanRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name          string
		contextLimit  int
		fixedInput    int
		outputReserve int
	}{
		{name: "zero context", contextLimit: 0, fixedInput: 0, outputReserve: 0},
		{name: "negative fixed input", contextLimit: 100, fixedInput: -1, outputReserve: 0},
		{name: "negative output reserve", contextLimit: 100, fixedInput: 0, outputReserve: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Plan(tt.contextLimit, tt.fixedInput, tt.outputReserve); err == nil {
				t.Fatal("Plan() error = nil, want validation error")
			}
		})
	}
}
