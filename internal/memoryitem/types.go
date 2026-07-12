package memoryitem

import "time"

type Operation string

const (
	OperationUpsert Operation = "upsert"
	OperationForget Operation = "forget"
)

type Kind string

const (
	KindIdentity    Kind = "identity"
	KindPreference  Kind = "preference"
	KindProjectFact Kind = "project_fact"
	KindGoal        Kind = "goal"
	KindConstraint  Kind = "constraint"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusForgotten Status = "forgotten"
)

// SourceMessage is an original chat message that can prove a memory candidate.
type SourceMessage struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

// Candidate is proposed by a model but must pass deterministic validation.
type Candidate struct {
	Operation        Operation `json:"operation"`
	Kind             Kind      `json:"kind"`
	Key              string    `json:"key"`
	Value            string    `json:"value"`
	Confidence       float64   `json:"confidence"`
	SourceMessageIDs []int64   `json:"source_message_ids"`
}

// Item is the current user-scoped fact persisted by the memory store.
type Item struct {
	ID        int64
	UserID    string
	Kind      Kind
	Key       string
	Value     string
	Status    Status
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
