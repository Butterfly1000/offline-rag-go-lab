package retrievalquality

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

type fixedReranker struct {
	scores []RerankScore
	err    error
}

func (r fixedReranker) Rank(_ context.Context, _ string, _ []Candidate) ([]RerankScore, error) {
	return append([]RerankScore(nil), r.scores...), r.err
}

func TestApplyRerankerOrdersByValidatedRelevance(t *testing.T) {
	input := rerankCandidates()
	result, err := ApplyReranker(context.Background(), fixedReranker{scores: []RerankScore{
		{CandidateID: "a", Relevance: 0.2},
		{CandidateID: "b", Relevance: 0.9},
		{CandidateID: "c", Relevance: 0.9},
	}}, "query", input)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.ChunkID)
	}
	if want := []string{"b", "c", "a"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order=%v, want %v", ids, want)
	}
	if !result.Used || len(result.Warnings) != 0 {
		t.Fatalf("result=%+v", result)
	}
	if result.Candidates[0].Trace["reranker_score"] != 0.9 ||
		result.Candidates[0].Trace["reranker_original_rank"] != 2 {
		t.Fatalf("trace=%v", result.Candidates[0].Trace)
	}
}

func TestApplyRerankerFallsBackOnBackendOrProtocolFailure(t *testing.T) {
	tests := []struct {
		name     string
		reranker fixedReranker
		want     string
	}{
		{name: "backend", reranker: fixedReranker{err: errors.New("timeout")}, want: "timeout"},
		{name: "missing", reranker: fixedReranker{scores: []RerankScore{{CandidateID: "a", Relevance: 1}}}, want: "missing"},
		{name: "duplicate", reranker: fixedReranker{scores: []RerankScore{
			{CandidateID: "a", Relevance: 1}, {CandidateID: "a", Relevance: 0.5}, {CandidateID: "c", Relevance: 0},
		}}, want: "duplicate"},
		{name: "unknown", reranker: fixedReranker{scores: []RerankScore{
			{CandidateID: "a", Relevance: 1}, {CandidateID: "b", Relevance: 0.5}, {CandidateID: "invented", Relevance: 0},
		}}, want: "unknown"},
		{name: "non-finite", reranker: fixedReranker{scores: []RerankScore{
			{CandidateID: "a", Relevance: 1}, {CandidateID: "b", Relevance: math.NaN()}, {CandidateID: "c", Relevance: 0},
		}}, want: "finite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := rerankCandidates()
			result, err := ApplyReranker(context.Background(), tt.reranker, "query", input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Used || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], tt.want) {
				t.Fatalf("result=%+v, want fallback warning %q", result, tt.want)
			}
			for i := range input {
				if result.Candidates[i].ChunkID != input[i].ChunkID || result.Candidates[i].Reason != "reranker_fallback" {
					t.Fatalf("fallback changed order/result=%+v", result)
				}
			}
		})
	}
}

func TestApplyRerankerRejectsInvalidInputAsHardFailure(t *testing.T) {
	input := rerankCandidates()
	input[1].KnowledgeScope = "other-scope"
	_, err := ApplyReranker(context.Background(), fixedReranker{}, "query", input)
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("error=%v, want scope hard failure", err)
	}
}

func rerankCandidates() []Candidate {
	return []Candidate{
		{KnowledgeScope: "scope-a", DocumentID: "d1", HeadingPath: "h1", ChunkID: "a", Content: "alpha", Score: 0.03, Trace: map[string]float64{"rrf_score": 0.03}},
		{KnowledgeScope: "scope-a", DocumentID: "d2", HeadingPath: "h2", ChunkID: "b", Content: "beta", Score: 0.02, Trace: map[string]float64{"rrf_score": 0.02}},
		{KnowledgeScope: "scope-a", DocumentID: "d3", HeadingPath: "h3", ChunkID: "c", Content: "gamma", Score: 0.01, Trace: map[string]float64{"rrf_score": 0.01}},
	}
}
