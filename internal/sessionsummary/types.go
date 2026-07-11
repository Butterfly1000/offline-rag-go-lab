package sessionsummary

import "time"

type SessionSummary struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	Content       string    `json:"content"`
	LastMessageID int64     `json:"last_message_id"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SourceMessage struct {
	ID      int64  `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TriggerInput struct {
	UnsummarizedMessages int `json:"unsummarized_messages"`
	UnsummarizedTokens   int `json:"unsummarized_tokens"`
	EvictedMessages      int `json:"evicted_messages"`
}

type TriggerReason string

const (
	ReasonNoEvictedMessages TriggerReason = "no_evicted_messages"
	ReasonBothThresholds    TriggerReason = "both_thresholds"
	ReasonMessageThreshold  TriggerReason = "message_threshold"
	ReasonTokenThreshold    TriggerReason = "token_threshold"
	ReasonBelowThreshold    TriggerReason = "below_threshold"
)

type TriggerDecision struct {
	ShouldSummarize bool          `json:"should_summarize"`
	Reason          TriggerReason `json:"reason"`
}
