package retrievalquality

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const rerankCandidateLimit = 20

type RerankStats struct {
	Calls            int           `json:"calls"`
	Used             int           `json:"used"`
	Fallbacks        int           `json:"fallbacks"`
	InputCandidates  int           `json:"input_candidates"`
	DiversitySkipped int           `json:"diversity_skipped"`
	LatencyP50       time.Duration `json:"latency_p50_ns"`
	LatencyP95       time.Duration `json:"latency_p95_ns"`
}

type RerankedStrategy struct {
	base      Strategy
	reranker  Reranker
	diversity DiversityLimits

	mu         sync.Mutex
	calls      int
	used       int
	fallbacks  int
	candidates int
	skipped    int
	latencies  []time.Duration
}

func NewRerankedStrategy(base Strategy, reranker Reranker, diversity DiversityLimits) (*RerankedStrategy, error) {
	if base == nil {
		return nil, fmt.Errorf("reranked base strategy is required")
	}
	if reranker == nil {
		return nil, fmt.Errorf("reranker is required")
	}
	if diversity.PerDocument <= 0 || diversity.PerHeading <= 0 {
		return nil, fmt.Errorf("diversity document and heading limits must be positive")
	}
	return &RerankedStrategy{base: base, reranker: reranker, diversity: diversity}, nil
}

func (s *RerankedStrategy) Name() string { return "hybrid_rerank_diversity" }

func (s *RerankedStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil || s.base == nil || s.reranker == nil {
		return SearchResult{}, fmt.Errorf("reranked strategy is not initialized")
	}
	query.Text = strings.TrimSpace(query.Text)
	query.KnowledgeScope = strings.TrimSpace(query.KnowledgeScope)
	if query.Text == "" || query.KnowledgeScope == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("reranked query text, scope, and positive limit are required")
	}
	started := time.Now()
	baseQuery := query
	baseQuery.Limit = rerankCandidateLimit
	baseResult, err := s.base.Search(ctx, baseQuery)
	if err != nil {
		return SearchResult{}, err
	}
	for _, candidate := range baseResult.Candidates {
		if strings.TrimSpace(candidate.KnowledgeScope) != query.KnowledgeScope {
			return SearchResult{}, &IntegrityError{Err: fmt.Errorf(
				"reranker candidate %q belongs to scope %q, expected %q",
				candidate.ChunkID, candidate.KnowledgeScope, query.KnowledgeScope,
			)}
		}
	}
	reranked, err := ApplyReranker(ctx, s.reranker, query.Text, baseResult.Candidates)
	if err != nil {
		return SearchResult{}, &IntegrityError{Err: fmt.Errorf("reranker input: %w", err)}
	}
	candidates, skipped, err := ApplyDiversity(reranked.Candidates, s.diversity)
	if err != nil {
		return SearchResult{}, &IntegrityError{Err: fmt.Errorf("reranker diversity: %w", err)}
	}
	if len(candidates) > query.Limit {
		candidates = candidates[:query.Limit]
	}
	s.record(reranked, len(skipped))
	warnings := append([]string(nil), baseResult.Warnings...)
	warnings = append(warnings, reranked.Warnings...)
	return SearchResult{
		Candidates: candidates,
		Abstained:  baseResult.Abstained,
		Duration:   time.Since(started),
		Warnings:   warnings,
	}, nil
}

func (s *RerankedStrategy) record(result RerankResult, skipped int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.candidates += len(result.Candidates)
	if result.Used {
		s.used++
	} else {
		s.fallbacks++
	}
	s.skipped += skipped
	s.latencies = append(s.latencies, result.Duration)
}

func (s *RerankedStrategy) Stats() RerankStats {
	if s == nil {
		return RerankStats{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return RerankStats{
		Calls:            s.calls,
		Used:             s.used,
		Fallbacks:        s.fallbacks,
		InputCandidates:  s.candidates,
		DiversitySkipped: s.skipped,
		LatencyP50:       percentile(s.latencies, 0.50),
		LatencyP95:       percentile(s.latencies, 0.95),
	}
}
