package documentingest

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type GoldenCase struct {
	CaseID            string   `json:"case_id"`
	Query             string   `json:"query"`
	KnowledgeScope    string   `json:"knowledge_scope"`
	ExpectedChunkIDs  []string `json:"expected_chunk_ids"`
	ForbiddenChunkIDs []string `json:"forbidden_chunk_ids"`
	Notes             string   `json:"notes,omitempty"`
}

type EvaluationHit struct{ ChunkID, KnowledgeScope string }
type QueryEmbedder interface {
	Embed(context.Context, string, []string) ([][]float32, error)
}
type ScopedSearcher interface {
	Search(context.Context, string, []float32, int) ([]EvaluationHit, error)
}

type CaseResult struct {
	CaseID            string   `json:"case_id"`
	RecallAt3         float64  `json:"recall_at_3"`
	MRRAt3            float64  `json:"mrr_at_3"`
	ScopeIsolated     bool     `json:"scope_isolated"`
	ForbiddenHits     []string `json:"forbidden_hits"`
	RetrievedChunkIDs []string `json:"retrieved_chunk_ids"`
}
type EvaluationReport struct {
	CaseCount      int          `json:"case_count"`
	MeanRecallAt3  float64      `json:"mean_recall_at_3"`
	MeanMRRAt3     float64      `json:"mean_mrr_at_3"`
	ScopeIsolation float64      `json:"scope_isolation"`
	Passed         bool         `json:"passed"`
	Cases          []CaseResult `json:"cases"`
}

func Evaluate(ctx context.Context, cases []GoldenCase, model string, embedder QueryEmbedder, searcher ScopedSearcher) (EvaluationReport, error) {
	if len(cases) < 10 {
		return EvaluationReport{}, fmt.Errorf("golden evaluation requires at least 10 cases, got %d", len(cases))
	}
	if embedder == nil || searcher == nil || strings.TrimSpace(model) == "" {
		return EvaluationReport{}, fmt.Errorf("embedding model, embedder, and searcher are required")
	}
	ordered := append([]GoldenCase(nil), cases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CaseID < ordered[j].CaseID })
	caseIDs := map[string]bool{}
	for i := range ordered {
		if err := validateGoldenCase(ordered[i], caseIDs); err != nil {
			return EvaluationReport{}, fmt.Errorf("golden case %d: %w", i, err)
		}
	}
	report := EvaluationReport{CaseCount: len(ordered), Passed: true, Cases: make([]CaseResult, 0, len(ordered))}
	isolated := 0
	for _, golden := range ordered {
		vectors, err := embedder.Embed(ctx, model, []string{golden.Query})
		if err != nil {
			return EvaluationReport{}, fmt.Errorf("embed case %s: %w", golden.CaseID, err)
		}
		if len(vectors) != 1 {
			return EvaluationReport{}, fmt.Errorf("case %s embedding count=%d, want 1", golden.CaseID, len(vectors))
		}
		if err := validateFiniteVector(vectors[0]); err != nil {
			return EvaluationReport{}, fmt.Errorf("case %s embedding: %w", golden.CaseID, err)
		}
		hits, err := searcher.Search(ctx, golden.KnowledgeScope, vectors[0], 3)
		if err != nil {
			return EvaluationReport{}, fmt.Errorf("search case %s: %w", golden.CaseID, err)
		}
		if len(hits) > 3 {
			return EvaluationReport{}, fmt.Errorf("case %s returned %d hits, limit is 3", golden.CaseID, len(hits))
		}
		result := CaseResult{CaseID: golden.CaseID, ScopeIsolated: true, ForbiddenHits: []string{}, RetrievedChunkIDs: []string{}}
		expected, forbidden, seen := stringSet(golden.ExpectedChunkIDs), stringSet(golden.ForbiddenChunkIDs), map[string]bool{}
		found, firstRank := 0, 0
		for rank, hit := range hits {
			if strings.TrimSpace(hit.KnowledgeScope) != golden.KnowledgeScope {
				return EvaluationReport{}, fmt.Errorf("case %s returned cross-scope hit %q", golden.CaseID, hit.KnowledgeScope)
			}
			id := strings.TrimSpace(hit.ChunkID)
			if id == "" || seen[id] {
				return EvaluationReport{}, fmt.Errorf("case %s returned empty or duplicate chunk ID %q", golden.CaseID, id)
			}
			seen[id] = true
			result.RetrievedChunkIDs = append(result.RetrievedChunkIDs, id)
			if expected[id] {
				found++
				if firstRank == 0 {
					firstRank = rank + 1
				}
			}
			if forbidden[id] {
				result.ForbiddenHits = append(result.ForbiddenHits, id)
			}
		}
		result.RecallAt3 = float64(found) / float64(len(expected))
		if firstRank > 0 {
			result.MRRAt3 = 1 / float64(firstRank)
		}
		sort.Strings(result.ForbiddenHits)
		isolated++
		report.MeanRecallAt3 += result.RecallAt3
		report.MeanMRRAt3 += result.MRRAt3
		if result.RecallAt3 != 1 || len(result.ForbiddenHits) > 0 {
			report.Passed = false
		}
		report.Cases = append(report.Cases, result)
	}
	report.MeanRecallAt3 /= float64(report.CaseCount)
	report.MeanMRRAt3 /= float64(report.CaseCount)
	report.ScopeIsolation = float64(isolated) / float64(report.CaseCount)
	return report, nil
}

func validateGoldenCase(item GoldenCase, caseIDs map[string]bool) error {
	item.CaseID, item.Query, item.KnowledgeScope = strings.TrimSpace(item.CaseID), strings.TrimSpace(item.Query), strings.TrimSpace(item.KnowledgeScope)
	if item.CaseID == "" || item.Query == "" || item.KnowledgeScope == "" {
		return fmt.Errorf("case_id, query, and knowledge_scope are required")
	}
	if caseIDs[item.CaseID] {
		return fmt.Errorf("duplicate case_id %q", item.CaseID)
	}
	caseIDs[item.CaseID] = true
	if len(item.ExpectedChunkIDs) < 1 || len(item.ExpectedChunkIDs) > 3 || len(item.ForbiddenChunkIDs) < 1 {
		return fmt.Errorf("expected IDs must contain 1-3 values and forbidden IDs must be non-empty")
	}
	expected := map[string]bool{}
	for _, id := range item.ExpectedChunkIDs {
		id = strings.TrimSpace(id)
		if id == "" || expected[id] {
			return fmt.Errorf("expected chunk IDs must be non-empty and unique")
		}
		expected[id] = true
	}
	forbidden := map[string]bool{}
	for _, id := range item.ForbiddenChunkIDs {
		id = strings.TrimSpace(id)
		if id == "" || forbidden[id] || expected[id] {
			return fmt.Errorf("forbidden chunk IDs must be unique and disjoint")
		}
		forbidden[id] = true
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[strings.TrimSpace(value)] = true
	}
	return result
}
