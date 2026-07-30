package retrievalquality

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type HybridMode string

const (
	HybridEvaluation HybridMode = "evaluation"
	HybridRuntime    HybridMode = "runtime"
	hybridLegLimit              = 20
)

type HybridStrategy struct {
	dense   Strategy
	sparse  Strategy
	weights FusionWeights
	mode    HybridMode
}

func NewHybridStrategy(dense, sparse Strategy, weights FusionWeights, mode HybridMode) (*HybridStrategy, error) {
	if dense == nil || sparse == nil {
		return nil, fmt.Errorf("hybrid dense and sparse strategies are required")
	}
	if mode != HybridEvaluation && mode != HybridRuntime {
		return nil, fmt.Errorf("hybrid mode %q is invalid", mode)
	}
	if err := validateFusionWeights(weights, 60); err != nil {
		return nil, err
	}
	return &HybridStrategy{dense: dense, sparse: sparse, weights: weights, mode: mode}, nil
}

func (s *HybridStrategy) Name() string { return "hybrid_weighted_rrf" }

func (s *HybridStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil || s.dense == nil || s.sparse == nil {
		return SearchResult{}, fmt.Errorf("hybrid strategy is not initialized")
	}
	if strings.TrimSpace(query.Text) == "" || strings.TrimSpace(query.KnowledgeScope) == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("hybrid query text, scope, and positive limit are required")
	}
	started := time.Now()
	legQuery := query
	legQuery.Limit = hybridLegLimit
	type legResult struct {
		name   string
		result SearchResult
		err    error
	}
	results := make(chan legResult, 2)
	go func() {
		result, err := s.dense.Search(ctx, legQuery)
		results <- legResult{name: "dense", result: result, err: err}
	}()
	go func() {
		result, err := s.sparse.Search(ctx, legQuery)
		results <- legResult{name: "sparse", result: result, err: err}
	}()
	var denseResult, sparseResult SearchResult
	var denseErr, sparseErr error
	for range 2 {
		result := <-results
		if result.name == "dense" {
			denseResult, denseErr = result.result, result.err
		} else {
			sparseResult, sparseErr = result.result, result.err
		}
	}
	if denseErr != nil {
		return SearchResult{}, fmt.Errorf("hybrid dense leg: %w", denseErr)
	}
	if sparseErr != nil {
		if s.mode != HybridRuntime || !IsInfrastructure(sparseErr) {
			return SearchResult{}, fmt.Errorf("hybrid sparse leg: %w", sparseErr)
		}
		if err := validateCandidateLeg("dense fallback", denseResult.Candidates); err != nil {
			return SearchResult{}, err
		}
		candidates := append([]Candidate(nil), denseResult.Candidates...)
		for i := range candidates {
			candidates[i].Reason = "dense_fallback"
		}
		if len(candidates) > query.Limit {
			candidates = candidates[:query.Limit]
		}
		warnings := append([]string(nil), denseResult.Warnings...)
		warnings = append(warnings, "sparse infrastructure fallback: "+sparseErr.Error())
		return SearchResult{Candidates: candidates, Duration: time.Since(started), Warnings: warnings}, nil
	}
	fused, err := FuseRRF(denseResult.Candidates, sparseResult.Candidates, s.weights, 60)
	if err != nil {
		return SearchResult{}, &IntegrityError{Err: fmt.Errorf("hybrid fusion: %w", err)}
	}
	if len(fused) > query.Limit {
		fused = fused[:query.Limit]
	}
	warnings := append([]string(nil), denseResult.Warnings...)
	warnings = append(warnings, sparseResult.Warnings...)
	return SearchResult{Candidates: fused, Duration: time.Since(started), Warnings: warnings}, nil
}
