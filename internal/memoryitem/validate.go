package memoryitem

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxMemoryKeyBytes   = 128
	maxMemoryValueRunes = 4096
)

// ValidateAndNormalizeCandidate treats model output as untrusted data. A
// candidate is accepted only when every cited source is an original user
// message from the extraction's user and session boundary.
func ValidateAndNormalizeCandidate(userID, sessionID string, candidate Candidate, messages []SourceMessage) (Candidate, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return Candidate{}, fmt.Errorf("user ID is required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Candidate{}, fmt.Errorf("session ID is required")
	}

	operation, err := normalizeOperation(candidate.Operation)
	if err != nil {
		return Candidate{}, err
	}
	kind, err := normalizeKind(candidate.Kind)
	if err != nil {
		return Candidate{}, err
	}
	key, err := normalizeMemoryKey(candidate.Key)
	if err != nil {
		return Candidate{}, err
	}
	if math.IsNaN(candidate.Confidence) || math.IsInf(candidate.Confidence, 0) || candidate.Confidence < 0 || candidate.Confidence > 1 {
		return Candidate{}, fmt.Errorf("memory confidence must be between 0 and 1: %v", candidate.Confidence)
	}

	value := strings.TrimSpace(candidate.Value)
	if operation == OperationUpsert && value == "" {
		return Candidate{}, fmt.Errorf("memory value is required for upsert")
	}
	if utf8.RuneCountInString(value) > maxMemoryValueRunes {
		return Candidate{}, fmt.Errorf("memory value is too long: maximum %d characters", maxMemoryValueRunes)
	}
	if operation == OperationForget {
		value = ""
	}

	sourceMessages, err := indexSourceMessages(messages)
	if err != nil {
		return Candidate{}, err
	}
	sourceIDs, err := validateSourceIDs(userID, sessionID, candidate.SourceMessageIDs, sourceMessages)
	if err != nil {
		return Candidate{}, err
	}

	candidate.Operation = operation
	candidate.Kind = kind
	candidate.Key = key
	candidate.Value = value
	candidate.SourceMessageIDs = sourceIDs
	return candidate, nil
}

func normalizeOperation(operation Operation) (Operation, error) {
	normalized := Operation(strings.ToLower(strings.TrimSpace(string(operation))))
	switch normalized {
	case OperationUpsert, OperationForget:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported memory operation %q", operation)
	}
}

func normalizeKind(kind Kind) (Kind, error) {
	normalized := Kind(strings.ToLower(strings.TrimSpace(string(kind))))
	switch normalized {
	case KindIdentity, KindPreference, KindProjectFact, KindGoal, KindConstraint:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported memory kind %q", kind)
	}
}

func normalizeMemoryKey(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", fmt.Errorf("memory key is required")
	}

	var builder strings.Builder
	lastUnderscore := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastUnderscore = false
		case r == '_' || r == '-' || unicode.IsSpace(r):
			if builder.Len() > 0 && !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		default:
			return "", fmt.Errorf("memory key %q contains unsupported character %q", raw, r)
		}
	}

	key := strings.Trim(builder.String(), "_")
	if key == "" {
		return "", fmt.Errorf("memory key is required")
	}
	if key[0] < 'a' || key[0] > 'z' {
		return "", fmt.Errorf("memory key %q must start with a letter", key)
	}
	if len(key) > maxMemoryKeyBytes {
		return "", fmt.Errorf("memory key is too long: maximum %d bytes", maxMemoryKeyBytes)
	}
	return key, nil
}

func indexSourceMessages(messages []SourceMessage) (map[int64]SourceMessage, error) {
	indexed := make(map[int64]SourceMessage, len(messages))
	for _, message := range messages {
		if message.ID <= 0 {
			return nil, fmt.Errorf("source input message ID must be positive: %d", message.ID)
		}
		if _, exists := indexed[message.ID]; exists {
			return nil, fmt.Errorf("duplicate source message ID %d", message.ID)
		}
		indexed[message.ID] = message
	}
	return indexed, nil
}

func validateSourceIDs(userID, sessionID string, sourceIDs []int64, messages map[int64]SourceMessage) ([]int64, error) {
	if len(sourceIDs) == 0 {
		return nil, fmt.Errorf("source message IDs are required")
	}

	validated := make([]int64, 0, len(sourceIDs))
	seen := make(map[int64]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if sourceID <= 0 {
			return nil, fmt.Errorf("source message ID must be positive: %d", sourceID)
		}
		if _, exists := seen[sourceID]; exists {
			continue
		}
		message, exists := messages[sourceID]
		if !exists {
			return nil, fmt.Errorf("source message %d is not in the extraction input", sourceID)
		}
		if strings.TrimSpace(message.UserID) != userID {
			return nil, fmt.Errorf("source message %d belongs to user %q, not %q", sourceID, message.UserID, userID)
		}
		if strings.TrimSpace(message.SessionID) != sessionID {
			return nil, fmt.Errorf("source message %d belongs to session %q, not %q", sourceID, message.SessionID, sessionID)
		}
		if strings.ToLower(strings.TrimSpace(message.Role)) != "user" {
			return nil, fmt.Errorf("source message %d must have role user", sourceID)
		}
		if strings.TrimSpace(message.Content) == "" {
			return nil, fmt.Errorf("source message %d content is required", sourceID)
		}
		seen[sourceID] = struct{}{}
		validated = append(validated, sourceID)
	}
	return validated, nil
}
