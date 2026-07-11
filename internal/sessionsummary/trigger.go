package sessionsummary

import "fmt"

type TriggerPolicy struct {
	minMessages int
	minTokens   int
}

func NewTriggerPolicy(minMessages int, minTokens int) (TriggerPolicy, error) {
	if minMessages <= 0 {
		return TriggerPolicy{}, fmt.Errorf("minimum messages must be positive: %d", minMessages)
	}
	if minTokens <= 0 {
		return TriggerPolicy{}, fmt.Errorf("minimum tokens must be positive: %d", minTokens)
	}
	return TriggerPolicy{minMessages: minMessages, minTokens: minTokens}, nil
}

// Decide requires actual recent-window eviction before threshold checks. This
// avoids summarizing messages whose original text still fits in the window.
func (p TriggerPolicy) Decide(input TriggerInput) (TriggerDecision, error) {
	if input.UnsummarizedMessages < 0 {
		return TriggerDecision{}, fmt.Errorf("unsummarized messages must not be negative: %d", input.UnsummarizedMessages)
	}
	if input.UnsummarizedTokens < 0 {
		return TriggerDecision{}, fmt.Errorf("unsummarized tokens must not be negative: %d", input.UnsummarizedTokens)
	}
	if input.EvictedMessages < 0 {
		return TriggerDecision{}, fmt.Errorf("evicted messages must not be negative: %d", input.EvictedMessages)
	}
	if input.EvictedMessages > input.UnsummarizedMessages {
		return TriggerDecision{}, fmt.Errorf(
			"evicted messages (%d) cannot exceed unsummarized messages (%d)",
			input.EvictedMessages,
			input.UnsummarizedMessages,
		)
	}
	if input.EvictedMessages == 0 {
		return TriggerDecision{Reason: ReasonNoEvictedMessages}, nil
	}

	messagesReached := input.UnsummarizedMessages >= p.minMessages
	tokensReached := input.UnsummarizedTokens >= p.minTokens
	switch {
	case messagesReached && tokensReached:
		return TriggerDecision{ShouldSummarize: true, Reason: ReasonBothThresholds}, nil
	case messagesReached:
		return TriggerDecision{ShouldSummarize: true, Reason: ReasonMessageThreshold}, nil
	case tokensReached:
		return TriggerDecision{ShouldSummarize: true, Reason: ReasonTokenThreshold}, nil
	default:
		return TriggerDecision{Reason: ReasonBelowThreshold}, nil
	}
}
