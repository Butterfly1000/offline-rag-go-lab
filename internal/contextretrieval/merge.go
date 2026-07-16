package contextretrieval

import (
	"fmt"
	"sort"
	"strings"
)

type MergeLimits struct {
	Memory    int
	Documents int
}

// Merge ranks each source independently. Scores from different collections
// are not treated as globally calibrated values.
func Merge(memoryHits, documentHits []Hit, limits MergeLimits) ([]Hit, error) {
	if limits.Memory < 0 || limits.Documents < 0 {
		return nil, fmt.Errorf("merge limits must not be negative: memory=%d documents=%d", limits.Memory, limits.Documents)
	}
	memory, err := sortedSourceHits(memoryHits, SourceMemory)
	if err != nil {
		return nil, err
	}
	documents, err := sortedSourceHits(documentHits, SourceDocument)
	if err != nil {
		return nil, err
	}

	merged := make([]Hit, 0, min(limits.Memory, len(memory))+min(limits.Documents, len(documents)))
	seenContent := make(map[string]struct{}, len(memory)+len(documents))
	appendUnique := func(candidates []Hit, limit int) {
		added := 0
		for _, hit := range candidates {
			if added >= limit {
				break
			}
			key := normalizedHitContent(hit.Content)
			if _, exists := seenContent[key]; exists {
				continue
			}
			seenContent[key] = struct{}{}
			merged = append(merged, hit)
			added++
		}
	}
	// Memory wins exact normalized-content duplicates because it is appended first.
	appendUnique(memory, limits.Memory)
	appendUnique(documents, limits.Documents)
	return merged, nil
}

func sortedSourceHits(input []Hit, source Source) ([]Hit, error) {
	validated := make([]Hit, 0, len(input))
	for index, hit := range input {
		hit, err := ValidateHit(hit)
		if err != nil {
			return nil, fmt.Errorf("validate %s hit %d: %w", source, index, err)
		}
		if hit.Source != source {
			return nil, fmt.Errorf("%s input hit %d has source %q", source, index, hit.Source)
		}
		validated = append(validated, hit)
	}
	sort.SliceStable(validated, func(i, j int) bool {
		if validated[i].Score != validated[j].Score {
			return validated[i].Score > validated[j].Score
		}
		return validated[i].ID < validated[j].ID
	})
	return validated, nil
}

func normalizedHitContent(content string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
}
