package retrievalquality

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type queryRecorder struct {
	denseScope  string
	sparseScope string
	dense       []Candidate
	sparse      []Candidate
	err         error
}

func (q *queryRecorder) QueryDense(_ context.Context, scope string, _ []float32, _ int) ([]Candidate, error) {
	q.denseScope = scope
	return append([]Candidate(nil), q.dense...), q.err
}

func (q *queryRecorder) QuerySparse(_ context.Context, scope string, _ SparseVector, _ int) ([]Candidate, error) {
	q.sparseScope = scope
	return append([]Candidate(nil), q.sparse...), q.err
}

func TestQdrantDenseStrategyEmbedsQueryAndPreservesScope(t *testing.T) {
	queryClient := &queryRecorder{dense: []Candidate{{KnowledgeScope: "scope-a", ChunkID: "a"}}}
	strategy, err := NewQdrantDenseStrategy(queryClient, mapEmbedder{vectors: map[string][]float32{
		"query": {1, 0},
	}}, "bge-m3")
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if queryClient.denseScope != "scope-a" || len(result.Candidates) != 1 {
		t.Fatalf("scope=%q result=%+v", queryClient.denseScope, result)
	}
}

func TestQdrantSparseStrategyEncodesQueryAndClassifiesInfrastructure(t *testing.T) {
	encoder, err := NewSparseEncoder(tokenMap{"query": {7}}, FieldStats{
		AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	queryClient := &queryRecorder{err: &InfrastructureError{Err: errors.New("down")}}
	strategy, err := NewQdrantSparseStrategy(queryClient, encoder)
	if err != nil {
		t.Fatal(err)
	}
	_, err = strategy.Search(context.Background(), Query{CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10})
	if err == nil || !IsInfrastructure(err) || !strings.Contains(err.Error(), "down") {
		t.Fatalf("error=%v, want infrastructure down", err)
	}
	if queryClient.sparseScope != "scope-a" {
		t.Fatalf("scope=%q", queryClient.sparseScope)
	}
}
