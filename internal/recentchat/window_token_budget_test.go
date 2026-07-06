package recentchat

import (
	"errors"
	"testing"
)

type fakeTokenCounter struct {
	counts map[string]int
	err    error
}

func (f fakeTokenCounter) CountText(text string) (int, []string, []int, error) {
	if f.err != nil {
		return 0, nil, nil, f.err
	}
	return f.counts[text], nil, nil, nil
}

func TestTokenBudgetWindowBuilderKeepsNewestMessagesWithinBudget(t *testing.T) {
	builder := NewTokenBudgetWindowBuilder(fakeTokenCounter{
		counts: map[string]int{
			"m1": 4,
			"m2": 6,
			"m3": 5,
			"m4": 3,
		},
	})

	in := []Message{
		{Content: "m1"},
		{Content: "m2"},
		{Content: "m3"},
		{Content: "m4"},
	}

	out, used, err := builder.Build(in, 8)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if used != 8 {
		t.Fatalf("unexpected used tokens: %d", used)
	}
	if len(out) != 2 || out[0].Content != "m3" || out[1].Content != "m4" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestTokenBudgetWindowBuilderKeepsSingleNewestMessageWhenItAloneExceedsBudget(t *testing.T) {
	builder := NewTokenBudgetWindowBuilder(fakeTokenCounter{
		counts: map[string]int{
			"m1": 100,
			"m2": 3,
		},
	})

	in := []Message{
		{Content: "m1"},
		{Content: "m2"},
	}

	out, used, err := builder.Build(in, 2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if used != 3 {
		t.Fatalf("unexpected used tokens: %d", used)
	}
	if len(out) != 1 || out[0].Content != "m2" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestTokenBudgetWindowBuilderReturnsErrorWhenCounterFails(t *testing.T) {
	builder := NewTokenBudgetWindowBuilder(fakeTokenCounter{err: errors.New("boom")})
	_, _, err := builder.Build([]Message{{Content: "m1"}}, 10)
	if err == nil {
		t.Fatal("expected error")
	}
}
