package retrievalquality

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type recordingStrategy struct {
	name   string
	result SearchResult
	err    error
	mu     sync.Mutex
	query  Query
}

func (s *recordingStrategy) Name() string { return s.name }

func (s *recordingStrategy) Search(_ context.Context, query Query) (SearchResult, error) {
	s.mu.Lock()
	s.query = query
	s.mu.Unlock()
	return s.result, s.err
}

func (s *recordingStrategy) recordedQuery() Query {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.query
}

func TestHybridUsesSameScopeTop20AndFusesDeterministically(t *testing.T) {
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a"},
	}}}
	sparse := &recordingStrategy{name: "sparse", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "b"},
	}}}
	hybrid, err := NewHybridStrategy(dense, sparse, FusionWeights{Dense: 1, Sparse: 1}, HybridEvaluation)
	if err != nil {
		t.Fatal(err)
	}
	result, err := hybrid.Search(context.Background(), Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dense.recordedQuery().KnowledgeScope != "scope-a" || sparse.recordedQuery().KnowledgeScope != "scope-a" {
		t.Fatalf("dense query=%+v sparse query=%+v", dense.recordedQuery(), sparse.recordedQuery())
	}
	if dense.recordedQuery().Limit != 20 || sparse.recordedQuery().Limit != 20 {
		t.Fatalf("leg limits dense=%d sparse=%d", dense.recordedQuery().Limit, sparse.recordedQuery().Limit)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].ChunkID != "a" {
		t.Fatalf("result=%+v", result)
	}
}

func TestHybridRuntimeFallsBackOnlyForSparseInfrastructureFailure(t *testing.T) {
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a", Reason: "dense"},
	}}}
	sparse := &recordingStrategy{name: "sparse", err: &InfrastructureError{Err: errors.New("qdrant down")}}
	hybrid, err := NewHybridStrategy(dense, sparse, FusionWeights{Dense: 1, Sparse: 1}, HybridRuntime)
	if err != nil {
		t.Fatal(err)
	}
	result, err := hybrid.Search(context.Background(), Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Reason != "dense_fallback" {
		t.Fatalf("fallback result=%+v", result)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "qdrant down") {
		t.Fatalf("warnings=%v", result.Warnings)
	}
}

func TestHybridEvaluationAndIntegrityFailuresAreHard(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode HybridMode
		err  error
	}{
		{name: "evaluation infrastructure", mode: HybridEvaluation, err: &InfrastructureError{Err: errors.New("down")}},
		{name: "runtime integrity", mode: HybridRuntime, err: &IntegrityError{Err: errors.New("cross-scope")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
				{KnowledgeScope: "scope-a", ChunkID: "a"},
			}}}
			sparse := &recordingStrategy{name: "sparse", err: tt.err}
			hybrid, err := NewHybridStrategy(dense, sparse, FusionWeights{Dense: 1, Sparse: 1}, tt.mode)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := hybrid.Search(context.Background(), Query{
				CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10,
			}); err == nil {
				t.Fatal("expected hard failure")
			}
		})
	}
}
