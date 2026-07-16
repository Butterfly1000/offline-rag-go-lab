package contextretrieval

import "fmt"

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type ContextSelection struct {
	Hits       []Hit
	Rendered   string
	UsedTokens int
	DroppedIDs []string
}

// SelectWithinTokenBudget counts the complete rendered block for every
// tentative addition. Oversized hits are skipped instead of being truncated.
func SelectWithinTokenBudget(candidates []Hit, maxTokens int, counter TextTokenCounter) (ContextSelection, error) {
	if maxTokens <= 0 {
		return ContextSelection{}, fmt.Errorf("context token budget must be positive: %d", maxTokens)
	}
	if counter == nil {
		return ContextSelection{}, fmt.Errorf("context token counter is required")
	}
	if len(candidates) == 0 {
		return ContextSelection{}, nil
	}

	selection := ContextSelection{}
	lastRetainedCount := 0
	for index, raw := range candidates {
		hit, err := ValidateHit(raw)
		if err != nil {
			return ContextSelection{}, fmt.Errorf("validate context candidate %d: %w", index, err)
		}
		tentative := appendCopy(selection.Hits, hit)
		rendered, err := RenderContext(tentative)
		if err != nil {
			return ContextSelection{}, err
		}
		count, _, _, err := counter.CountText(rendered)
		if err != nil {
			return ContextSelection{}, fmt.Errorf("count tentative context for %s: %w", hit.ID, err)
		}
		if count < 0 {
			return ContextSelection{}, fmt.Errorf("token counter returned negative count %d", count)
		}
		if count > maxTokens {
			selection.DroppedIDs = append(selection.DroppedIDs, hit.ID)
			continue
		}
		selection.Hits = tentative
		selection.Rendered = rendered
		selection.UsedTokens = count
		lastRetainedCount = count
	}
	if len(selection.Hits) == 0 {
		return selection, nil
	}

	finalRendered, err := RenderContext(selection.Hits)
	if err != nil {
		return ContextSelection{}, err
	}
	finalCount, _, _, err := counter.CountText(finalRendered)
	if err != nil {
		return ContextSelection{}, fmt.Errorf("count final context: %w", err)
	}
	if finalCount != lastRetainedCount {
		return ContextSelection{}, fmt.Errorf("token counter is not deterministic: tentative=%d final=%d", lastRetainedCount, finalCount)
	}
	selection.Rendered = finalRendered
	selection.UsedTokens = finalCount
	return selection, nil
}

func appendCopy(hits []Hit, hit Hit) []Hit {
	result := make([]Hit, len(hits), len(hits)+1)
	copy(result, hits)
	result = append(result, hit)
	return result
}
