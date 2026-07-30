package retrievalquality

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type mapEmbedder struct {
	vectors map[string][]float32
}

func (e mapEmbedder) Embed(_ context.Context, _ string, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = append([]float32(nil), e.vectors[text]...)
	}
	return result, nil
}

func TestDenseMemoryFiltersScopeBeforeStableCosineRanking(t *testing.T) {
	corpus := []Chunk{
		{ChunkID: "b", KnowledgeScope: "scope-a", Content: "second"},
		{ChunkID: "a", KnowledgeScope: "scope-a", Content: "first"},
		{ChunkID: "foreign", KnowledgeScope: "scope-b", Content: "foreign"},
	}
	embedder := mapEmbedder{vectors: map[string][]float32{
		"second":  {1, 0},
		"first":   {1, 0},
		"foreign": {1, 0},
		"query":   {1, 0},
	}}
	strategy, err := NewDenseMemoryStrategy(context.Background(), embedder, "test-model", corpus)
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.ChunkID)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v, want %v", ids, want)
	}
}

func TestDenseMemoryRejectsNonFiniteOrMismatchedVectors(t *testing.T) {
	corpus := []Chunk{
		{ChunkID: "a", KnowledgeScope: "scope-a", Content: "first"},
		{ChunkID: "b", KnowledgeScope: "scope-a", Content: "second"},
	}
	_, err := NewDenseMemoryStrategy(context.Background(), mapEmbedder{vectors: map[string][]float32{
		"first": {1, 0}, "second": {1},
	}}, "test-model", corpus)
	if err == nil || !strings.Contains(err.Error(), "dimension") {
		t.Fatalf("error=%v, want dimension failure", err)
	}
}
