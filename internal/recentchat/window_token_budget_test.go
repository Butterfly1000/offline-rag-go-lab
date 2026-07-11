package recentchat

import (
	"errors"
	"testing"

	"offline-rag-go-lab/internal/chatprompt"
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

func TestFormattedTokenWindowCountsRoleAndBoundaries(t *testing.T) {
	formatter := chatprompt.QwenFormatter{}
	formattedUser, err := formatter.FormatMessage(chatprompt.Message{Role: "user", Content: "same"})
	if err != nil {
		t.Fatalf("format user: %v", err)
	}
	formattedAssistant, err := formatter.FormatMessage(chatprompt.Message{Role: "assistant", Content: "same"})
	if err != nil {
		t.Fatalf("format assistant: %v", err)
	}

	builder := NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{
		counts: map[string]int{
			formattedUser:      6,
			formattedAssistant: 4,
		},
	}, formatter)

	out, used, err := builder.Build([]Message{
		{Role: RoleUser, Content: "same"},
		{Role: RoleAssistant, Content: "same"},
	}, 4)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if used != 4 {
		t.Fatalf("Build() used = %d, want 4", used)
	}
	if len(out) != 1 || out[0].Role != RoleAssistant {
		t.Fatalf("Build() output = %#v, want newest assistant message", out)
	}
}

func TestFormattedTokenWindowDoesNotForceOversizedNewestMessage(t *testing.T) {
	formatter := chatprompt.QwenFormatter{}
	formatted, err := formatter.FormatMessage(chatprompt.Message{Role: "assistant", Content: "large"})
	if err != nil {
		t.Fatalf("format assistant: %v", err)
	}
	builder := NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{
		counts: map[string]int{formatted: 10},
	}, formatter)

	out, used, err := builder.Build([]Message{{Role: RoleAssistant, Content: "large"}}, 9)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if used != 0 || len(out) != 0 {
		t.Fatalf("Build() output = %#v, used = %d; want empty strict window", out, used)
	}
}

func TestFormattedTokenWindowReturnsEmptyForZeroBudget(t *testing.T) {
	builder := NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{}, chatprompt.QwenFormatter{})

	out, used, err := builder.Build([]Message{{Role: RoleUser, Content: "history"}}, 0)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if used != 0 || len(out) != 0 {
		t.Fatalf("Build() output = %#v, used = %d; want empty zero-budget window", out, used)
	}
}

func TestFormattedTokenWindowReturnsRoleFormattingError(t *testing.T) {
	builder := NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{}, chatprompt.QwenFormatter{})

	_, _, err := builder.Build([]Message{{Role: MessageRole("unknown"), Content: "x"}}, 10)
	if err == nil {
		t.Fatal("Build() error = nil, want unsupported role error")
	}
}
