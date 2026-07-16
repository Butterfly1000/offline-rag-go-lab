package contextretrieval

import (
	"fmt"
	"math"
	"strings"
)

// ValidateHit normalizes a retrieval result and enforces the ownership fields
// required by its source. It must run even when Qdrant already applied filters.
func ValidateHit(hit Hit) (Hit, error) {
	hit.Source = Source(strings.TrimSpace(string(hit.Source)))
	hit.ID = strings.TrimSpace(hit.ID)
	hit.Content = strings.TrimSpace(hit.Content)
	hit.UserID = strings.TrimSpace(hit.UserID)
	hit.KnowledgeScope = strings.TrimSpace(hit.KnowledgeScope)
	hit.Kind = strings.TrimSpace(hit.Kind)
	hit.Title = strings.TrimSpace(hit.Title)
	hit.SourceRef = strings.TrimSpace(hit.SourceRef)

	if hit.ID == "" {
		return Hit{}, fmt.Errorf("retrieval hit ID is required")
	}
	if hit.Content == "" {
		return Hit{}, fmt.Errorf("retrieval hit content is required")
	}
	if math.IsNaN(hit.Score) || math.IsInf(hit.Score, 0) {
		return Hit{}, fmt.Errorf("retrieval hit score must be finite")
	}

	switch hit.Source {
	case SourceMemory:
		if hit.UserID == "" {
			return Hit{}, fmt.Errorf("memory hit user_id is required")
		}
		if hit.KnowledgeScope != "" {
			return Hit{}, fmt.Errorf("memory hit must not carry knowledge_scope")
		}
	case SourceDocument:
		if hit.KnowledgeScope == "" {
			return Hit{}, fmt.Errorf("document hit knowledge_scope is required")
		}
		if hit.UserID != "" {
			return Hit{}, fmt.Errorf("document hit must not carry user_id")
		}
	default:
		return Hit{}, fmt.Errorf("unknown retrieval source %q", hit.Source)
	}

	metadata, err := copyMetadata(hit.Metadata)
	if err != nil {
		return Hit{}, err
	}
	hit.Metadata = metadata
	return hit, nil
}

func copyMetadata(input map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	output := make(map[string]string, len(input))
	for rawKey, rawValue := range input {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return nil, fmt.Errorf("retrieval hit metadata key is required")
		}
		if _, exists := output[key]; exists {
			return nil, fmt.Errorf("retrieval hit metadata repeats normalized key %q", key)
		}
		output[key] = strings.TrimSpace(rawValue)
	}
	return output, nil
}
