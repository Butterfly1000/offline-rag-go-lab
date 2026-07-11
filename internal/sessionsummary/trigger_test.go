package sessionsummary

import (
	"strings"
	"testing"
)

func TestTriggerPolicyRequiresEvictedMessages(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)

	decision, err := policy.Decide(TriggerInput{
		UnsummarizedMessages: 10,
		UnsummarizedTokens:   5000,
		EvictedMessages:      0,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision.ShouldSummarize || decision.Reason != ReasonNoEvictedMessages {
		t.Fatalf("Decide() = %+v, want no-eviction decision", decision)
	}
}

func TestTriggerPolicyTriggersAtMessageThreshold(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)

	decision, err := policy.Decide(TriggerInput{
		UnsummarizedMessages: 8,
		UnsummarizedTokens:   1000,
		EvictedMessages:      2,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !decision.ShouldSummarize || decision.Reason != ReasonMessageThreshold {
		t.Fatalf("Decide() = %+v, want message-threshold decision", decision)
	}
}

func TestTriggerPolicyTriggersAtTokenThreshold(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)

	decision, err := policy.Decide(TriggerInput{
		UnsummarizedMessages: 3,
		UnsummarizedTokens:   3000,
		EvictedMessages:      1,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !decision.ShouldSummarize || decision.Reason != ReasonTokenThreshold {
		t.Fatalf("Decide() = %+v, want token-threshold decision", decision)
	}
}

func TestTriggerPolicyReportsBothThresholds(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)

	decision, err := policy.Decide(TriggerInput{
		UnsummarizedMessages: 9,
		UnsummarizedTokens:   3000,
		EvictedMessages:      3,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !decision.ShouldSummarize || decision.Reason != ReasonBothThresholds {
		t.Fatalf("Decide() = %+v, want both-thresholds decision", decision)
	}
}

func TestTriggerPolicyDoesNotTriggerBelowThresholds(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)

	decision, err := policy.Decide(TriggerInput{
		UnsummarizedMessages: 3,
		UnsummarizedTokens:   1000,
		EvictedMessages:      1,
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if decision.ShouldSummarize || decision.Reason != ReasonBelowThreshold {
		t.Fatalf("Decide() = %+v, want below-threshold decision", decision)
	}
}

func TestNewTriggerPolicyRejectsNonPositiveThresholds(t *testing.T) {
	for _, test := range []struct {
		name        string
		minMessages int
		minTokens   int
	}{
		{name: "zero messages", minMessages: 0, minTokens: 2048},
		{name: "negative messages", minMessages: -1, minTokens: 2048},
		{name: "zero tokens", minMessages: 8, minTokens: 0},
		{name: "negative tokens", minMessages: 8, minTokens: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewTriggerPolicy(test.minMessages, test.minTokens)
			if err == nil {
				t.Fatal("NewTriggerPolicy() error = nil, want validation error")
			}
		})
	}
}

func TestTriggerPolicyRejectsInvalidInput(t *testing.T) {
	policy := mustTriggerPolicy(t, 8, 2048)
	for _, test := range []struct {
		name  string
		input TriggerInput
		want  string
	}{
		{name: "negative messages", input: TriggerInput{UnsummarizedMessages: -1}, want: "unsummarized messages"},
		{name: "negative tokens", input: TriggerInput{UnsummarizedTokens: -1}, want: "unsummarized tokens"},
		{name: "negative evicted", input: TriggerInput{EvictedMessages: -1}, want: "evicted messages"},
		{
			name: "evicted exceeds unsummarized",
			input: TriggerInput{
				UnsummarizedMessages: 2,
				EvictedMessages:      3,
			},
			want: "cannot exceed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := policy.Decide(test.input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Decide() error = %v, want %q", err, test.want)
			}
		})
	}
}

func mustTriggerPolicy(t *testing.T, minMessages int, minTokens int) TriggerPolicy {
	t.Helper()
	policy, err := NewTriggerPolicy(minMessages, minTokens)
	if err != nil {
		t.Fatalf("NewTriggerPolicy() error = %v", err)
	}
	return policy
}
