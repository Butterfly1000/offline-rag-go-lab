package memoryitem

import (
	"math"
	"strings"
	"testing"
)

func TestValidateAndNormalizeCandidate(t *testing.T) {
	messages := []SourceMessage{
		{
			ID:        101,
			SessionID: "memory-validation",
			UserID:    "u-001",
			Role:      "user",
			Content:   "这个项目使用 Go。",
		},
		{
			ID:        102,
			SessionID: "memory-validation",
			UserID:    "u-001",
			Role:      "user",
			Content:   "代码示例保持简单。",
		},
	}

	got, err := ValidateAndNormalizeCandidate("u-001", "memory-validation", Candidate{
		Operation:        OperationUpsert,
		Kind:             KindProjectFact,
		Key:              " Implementation Language ",
		Value:            " Go ",
		Confidence:       0.95,
		SourceMessageIDs: []int64{102, 101, 102},
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "implementation_language" {
		t.Fatalf("normalized key = %q, want implementation_language", got.Key)
	}
	if got.Value != "Go" {
		t.Fatalf("normalized value = %q, want Go", got.Value)
	}
	if len(got.SourceMessageIDs) != 2 || got.SourceMessageIDs[0] != 102 || got.SourceMessageIDs[1] != 101 {
		t.Fatalf("source IDs = %v, want [102 101]", got.SourceMessageIDs)
	}
}

func TestValidateAndNormalizeForgetClearsOptionalValue(t *testing.T) {
	messages := []SourceMessage{{
		ID:        101,
		SessionID: "memory-validation",
		UserID:    "u-001",
		Role:      "user",
		Content:   "请忘掉我之前说的编辑器偏好。",
	}}

	got, err := ValidateAndNormalizeCandidate("u-001", "memory-validation", Candidate{
		Operation:        OperationForget,
		Kind:             KindPreference,
		Key:              " editor ",
		Value:            "Vim",
		Confidence:       1,
		SourceMessageIDs: []int64{101},
	}, messages)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "" {
		t.Fatalf("forget value = %q, want empty", got.Value)
	}
}

func TestValidateAndNormalizeCandidateRejectsInvalidInput(t *testing.T) {
	validMessages := []SourceMessage{{
		ID:        101,
		SessionID: "memory-validation",
		UserID:    "u-001",
		Role:      "user",
		Content:   "这个项目使用 Go。",
	}}
	validCandidate := Candidate{
		Operation:        OperationUpsert,
		Kind:             KindProjectFact,
		Key:              "implementation_language",
		Value:            "Go",
		Confidence:       0.9,
		SourceMessageIDs: []int64{101},
	}

	tests := []struct {
		name      string
		userID    string
		sessionID string
		candidate Candidate
		messages  []SourceMessage
		wantError string
	}{
		{name: "empty user", userID: " ", sessionID: "memory-validation", candidate: validCandidate, messages: validMessages, wantError: "user ID is required"},
		{name: "empty session", userID: "u-001", sessionID: " ", candidate: validCandidate, messages: validMessages, wantError: "session ID is required"},
		{name: "unknown operation", userID: "u-001", sessionID: "memory-validation", candidate: replaceOperation(validCandidate, Operation("merge")), messages: validMessages, wantError: "unsupported memory operation"},
		{name: "unknown kind", userID: "u-001", sessionID: "memory-validation", candidate: replaceKind(validCandidate, Kind("temporary")), messages: validMessages, wantError: "unsupported memory kind"},
		{name: "invalid key", userID: "u-001", sessionID: "memory-validation", candidate: replaceKey(validCandidate, "bad/key"), messages: validMessages, wantError: "memory key"},
		{name: "key too long", userID: "u-001", sessionID: "memory-validation", candidate: replaceKey(validCandidate, strings.Repeat("a", 129)), messages: validMessages, wantError: "memory key is too long"},
		{name: "empty upsert value", userID: "u-001", sessionID: "memory-validation", candidate: replaceValue(validCandidate, " "), messages: validMessages, wantError: "memory value is required"},
		{name: "value too long", userID: "u-001", sessionID: "memory-validation", candidate: replaceValue(validCandidate, strings.Repeat("a", 4097)), messages: validMessages, wantError: "memory value is too long"},
		{name: "confidence below zero", userID: "u-001", sessionID: "memory-validation", candidate: replaceConfidence(validCandidate, -0.1), messages: validMessages, wantError: "confidence"},
		{name: "confidence above one", userID: "u-001", sessionID: "memory-validation", candidate: replaceConfidence(validCandidate, 1.1), messages: validMessages, wantError: "confidence"},
		{name: "confidence nan", userID: "u-001", sessionID: "memory-validation", candidate: replaceConfidence(validCandidate, math.NaN()), messages: validMessages, wantError: "confidence"},
		{name: "no sources", userID: "u-001", sessionID: "memory-validation", candidate: replaceSources(validCandidate, nil), messages: validMessages, wantError: "source message IDs are required"},
		{name: "non-positive source", userID: "u-001", sessionID: "memory-validation", candidate: replaceSources(validCandidate, []int64{0}), messages: validMessages, wantError: "source message ID must be positive"},
		{name: "unknown source", userID: "u-001", sessionID: "memory-validation", candidate: replaceSources(validCandidate, []int64{999}), messages: validMessages, wantError: "source message 999 is not in the extraction input"},
		{
			name: "cross user source", userID: "u-001", sessionID: "memory-validation", candidate: validCandidate,
			messages:  []SourceMessage{{ID: 101, SessionID: "memory-validation", UserID: "u-002", Role: "user", Content: "使用 Go。"}},
			wantError: "source message 101 belongs to user",
		},
		{
			name: "cross session source", userID: "u-001", sessionID: "memory-validation", candidate: validCandidate,
			messages:  []SourceMessage{{ID: 101, SessionID: "another-session", UserID: "u-001", Role: "user", Content: "使用 Go。"}},
			wantError: "source message 101 belongs to session",
		},
		{
			name: "assistant only source", userID: "u-001", sessionID: "memory-validation", candidate: validCandidate,
			messages:  []SourceMessage{{ID: 101, SessionID: "memory-validation", UserID: "u-001", Role: "assistant", Content: "你使用 Go。"}},
			wantError: "source message 101 must have role user",
		},
		{
			name: "empty source content", userID: "u-001", sessionID: "memory-validation", candidate: validCandidate,
			messages:  []SourceMessage{{ID: 101, SessionID: "memory-validation", UserID: "u-001", Role: "user", Content: " "}},
			wantError: "source message 101 content is required",
		},
		{
			name: "duplicate input message ID", userID: "u-001", sessionID: "memory-validation", candidate: validCandidate,
			messages: []SourceMessage{
				{ID: 101, SessionID: "memory-validation", UserID: "u-001", Role: "user", Content: "使用 Go。"},
				{ID: 101, SessionID: "memory-validation", UserID: "u-001", Role: "user", Content: "使用 Rust。"},
			},
			wantError: "duplicate source message ID 101",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndNormalizeCandidate(tt.userID, tt.sessionID, tt.candidate, tt.messages)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func replaceOperation(candidate Candidate, operation Operation) Candidate {
	candidate.Operation = operation
	return candidate
}

func replaceKind(candidate Candidate, kind Kind) Candidate {
	candidate.Kind = kind
	return candidate
}

func replaceKey(candidate Candidate, key string) Candidate {
	candidate.Key = key
	return candidate
}

func replaceValue(candidate Candidate, value string) Candidate {
	candidate.Value = value
	return candidate
}

func replaceConfidence(candidate Candidate, confidence float64) Candidate {
	candidate.Confidence = confidence
	return candidate
}

func replaceSources(candidate Candidate, sources []int64) Candidate {
	candidate.SourceMessageIDs = sources
	return candidate
}
