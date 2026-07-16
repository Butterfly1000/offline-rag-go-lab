package contextretrieval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"offline-rag-go-lab/internal/memoryitem"
)

type fakeMemoryVectorSearcher struct {
	results []memoryitem.SearchResult
	err     error
}

func (f fakeMemoryVectorSearcher) Search(context.Context, string, memoryitem.Kind, []float32, int) ([]memoryitem.SearchResult, error) {
	return f.results, f.err
}

func TestMemoryQdrantSearcherConvertsAndValidatesResults(t *testing.T) {
	adapter := NewMemoryQdrantSearcher(fakeMemoryVectorSearcher{results: []memoryitem.SearchResult{{
		ItemID: 7, Score: 0.91, UserID: "u-001", Kind: memoryitem.KindProjectFact,
		Key: "implementation_language", Value: "Go", Version: 3,
	}}})
	hits, err := adapter.Search(context.Background(), "u-001", []float32{0.1}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "memory:7" || hits[0].UserID != "u-001" || hits[0].Kind != "project_fact" {
		t.Fatalf("hits = %#v", hits)
	}
	if hits[0].Content != "project_fact/implementation_language: Go" || hits[0].Metadata["key"] != "implementation_language" || hits[0].Metadata["version"] != "3" {
		t.Fatalf("hit content/metadata = %#v", hits[0])
	}
}

func TestMemoryQdrantSearcherClassifiesResultAndRequestFailures(t *testing.T) {
	tests := []struct {
		name  string
		index fakeMemoryVectorSearcher
		want  string
		infra bool
	}{
		{
			name: "cross user",
			index: fakeMemoryVectorSearcher{results: []memoryitem.SearchResult{{
				ItemID: 7, Score: 0.9, UserID: "u-002", Kind: memoryitem.KindProjectFact,
				Key: "language", Value: "Go", Version: 1,
			}}},
			want: "belongs to user",
		},
		{
			name:  "Qdrant data",
			index: fakeMemoryVectorSearcher{err: &memoryitem.QdrantDataError{Err: errors.New("bad payload")}},
			want:  "bad payload",
		},
		{
			name:  "HTTP",
			index: fakeMemoryVectorSearcher{err: errors.New("connection refused")},
			want:  "connection refused",
			infra: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMemoryQdrantSearcher(tt.index).Search(context.Background(), "u-001", []float32{0.1}, 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Search() error = %v, want %q", err, tt.want)
			}
			if IsInfrastructureFailure(err) != tt.infra {
				t.Fatalf("IsInfrastructureFailure(%v) = %t, want %t", err, IsInfrastructureFailure(err), tt.infra)
			}
		})
	}
}
