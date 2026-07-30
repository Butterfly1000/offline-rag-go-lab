package retrievalquality

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type RerankScore struct {
	CandidateID string  `json:"candidate_id"`
	Relevance   float64 `json:"relevance"`
}

type Reranker interface {
	Rank(context.Context, string, []Candidate) ([]RerankScore, error)
}

type RerankResult struct {
	Candidates []Candidate   `json:"candidates"`
	Warnings   []string      `json:"warnings,omitempty"`
	Used       bool          `json:"used"`
	Duration   time.Duration `json:"duration_ns"`
}

func ApplyReranker(ctx context.Context, reranker Reranker, query string, candidates []Candidate) (RerankResult, error) {
	if reranker == nil {
		return RerankResult{}, fmt.Errorf("reranker is required")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return RerankResult{}, fmt.Errorf("reranker query is required")
	}
	if err := validateCandidateLeg("reranker input", candidates); err != nil {
		return RerankResult{}, err
	}
	if len(candidates) == 0 {
		return RerankResult{Candidates: []Candidate{}}, nil
	}
	started := time.Now()
	scores, err := reranker.Rank(ctx, query, cloneCandidates(candidates))
	if err != nil {
		return rerankerFallback(candidates, time.Since(started), err.Error()), nil
	}
	byID := make(map[string]float64, len(scores))
	inputIDs := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		inputIDs[candidate.ChunkID] = true
	}
	for _, score := range scores {
		id := strings.TrimSpace(score.CandidateID)
		if !inputIDs[id] {
			return rerankerFallback(candidates, time.Since(started), fmt.Sprintf("unknown candidate ID %q", id)), nil
		}
		if _, exists := byID[id]; exists {
			return rerankerFallback(candidates, time.Since(started), fmt.Sprintf("duplicate candidate ID %q", id)), nil
		}
		if math.IsNaN(score.Relevance) || math.IsInf(score.Relevance, 0) {
			return rerankerFallback(candidates, time.Since(started), fmt.Sprintf("candidate %q relevance must be finite", id)), nil
		}
		byID[id] = score.Relevance
	}
	if len(byID) != len(candidates) {
		var missing []string
		for id := range inputIDs {
			if _, exists := byID[id]; !exists {
				missing = append(missing, id)
			}
		}
		sort.Strings(missing)
		return rerankerFallback(candidates, time.Since(started), fmt.Sprintf("missing candidate IDs %v", missing)), nil
	}
	type rankedCandidate struct {
		candidate    Candidate
		originalRank int
	}
	ranked := make([]rankedCandidate, len(candidates))
	for i, candidate := range cloneCandidates(candidates) {
		relevance := byID[candidate.ChunkID]
		if candidate.Trace == nil {
			candidate.Trace = map[string]float64{}
		}
		candidate.Trace["reranker_score"] = relevance
		candidate.Trace["reranker_original_rank"] = float64(i + 1)
		candidate.Score = relevance
		candidate.Reason = "reranked"
		ranked[i] = rankedCandidate{candidate: candidate, originalRank: i + 1}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].candidate.Score != ranked[j].candidate.Score {
			return ranked[i].candidate.Score > ranked[j].candidate.Score
		}
		if ranked[i].originalRank != ranked[j].originalRank {
			return ranked[i].originalRank < ranked[j].originalRank
		}
		return ranked[i].candidate.ChunkID < ranked[j].candidate.ChunkID
	})
	result := make([]Candidate, len(ranked))
	for i := range ranked {
		result[i] = ranked[i].candidate
	}
	return RerankResult{Candidates: result, Used: true, Duration: time.Since(started)}, nil
}

func rerankerFallback(candidates []Candidate, duration time.Duration, reason string) RerankResult {
	result := cloneCandidates(candidates)
	for i := range result {
		result[i].Reason = "reranker_fallback"
	}
	return RerankResult{
		Candidates: result, Warnings: []string{"reranker fallback: " + reason}, Duration: duration,
	}
}

func cloneCandidates(input []Candidate) []Candidate {
	result := make([]Candidate, len(input))
	for i, candidate := range input {
		result[i] = candidate
		if candidate.Trace != nil {
			result[i].Trace = make(map[string]float64, len(candidate.Trace))
			for key, value := range candidate.Trace {
				result[i].Trace[key] = value
			}
		}
	}
	return result
}
