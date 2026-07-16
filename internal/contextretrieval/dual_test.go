package contextretrieval

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordingEmbedder struct {
	mu      sync.Mutex
	calls   int
	vectors [][]float32
	err     error
}

func (e *recordingEmbedder) Embed(context.Context, string, []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	return e.vectors, e.err
}

type memorySearchFunc func(context.Context, string, []float32, int) ([]Hit, error)

func (f memorySearchFunc) Search(ctx context.Context, userID string, vector []float32, limit int) ([]Hit, error) {
	return f(ctx, userID, vector, limit)
}

type documentSearchFunc func(context.Context, string, []float32, int) ([]Hit, error)

func (f documentSearchFunc) Search(ctx context.Context, scope string, vector []float32, limit int) ([]Hit, error) {
	return f(ctx, scope, vector, limit)
}

func TestDualRetrieverEmbedsOnceAndSearchesBothSourcesConcurrently(t *testing.T) {
	embedder := &recordingEmbedder{vectors: [][]float32{{0.1, 0.2}}}
	started := make(chan Source, 2)
	release := make(chan struct{})
	var mu sync.Mutex
	seen := map[Source][]float32{}
	search := func(source Source, hit Hit) func(context.Context, string, []float32, int) ([]Hit, error) {
		return func(_ context.Context, _ string, vector []float32, _ int) ([]Hit, error) {
			mu.Lock()
			seen[source] = append([]float32(nil), vector...)
			mu.Unlock()
			started <- source
			<-release
			return []Hit{hit}, nil
		}
	}
	retriever := NewDualRetriever(
		embedder, "bge-m3",
		memorySearchFunc(search(SourceMemory, validMemoryHit())),
		documentSearchFunc(search(SourceDocument, validDocumentHit())),
	)

	done := make(chan struct{})
	var result DualResult
	var retrieveErr error
	go func() {
		result, retrieveErr = retriever.Retrieve(context.Background(), validDualRequest())
		close(done)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("searches did not overlap")
		}
	}
	close(release)
	<-done
	if retrieveErr != nil || len(result.MemoryHits) != 1 || len(result.DocumentHits) != 1 {
		t.Fatalf("Retrieve() result=%#v error=%v", result, retrieveErr)
	}
	if embedder.calls != 1 || !reflect.DeepEqual(seen[SourceMemory], seen[SourceDocument]) {
		t.Fatalf("embed calls=%d vectors=%#v", embedder.calls, seen)
	}
}

func TestDualRetrieverFailureIsolation(t *testing.T) {
	infra := InfrastructureFailure(SourceMemory, errors.New("memory timeout"))
	integrity := IntegrityFailure(SourceDocument, errors.New("wrong scope"))
	tests := []struct {
		name         string
		memoryErr    error
		documentErr  error
		wantWarnings int
		wantHard     string
	}{
		{name: "one infrastructure", memoryErr: infra, wantWarnings: 1},
		{name: "both infrastructure", memoryErr: infra, documentErr: InfrastructureFailure(SourceDocument, errors.New("document timeout")), wantWarnings: 2},
		{name: "integrity", documentErr: integrity, wantHard: "wrong scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retriever := NewDualRetriever(
				&recordingEmbedder{vectors: [][]float32{{0.1}}}, "bge-m3",
				memorySearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
					return []Hit{validMemoryHit()}, tt.memoryErr
				}),
				documentSearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
					return []Hit{validDocumentHit()}, tt.documentErr
				}),
			)
			result, err := retriever.Retrieve(context.Background(), validDualRequest())
			if tt.wantHard != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantHard) {
					t.Fatalf("Retrieve() error = %v, want %q", err, tt.wantHard)
				}
				return
			}
			if err != nil || len(result.Warnings) != tt.wantWarnings {
				t.Fatalf("Retrieve() result=%#v error=%v", result, err)
			}
		})
	}
}

func TestDualRetrieverEmbeddingFailureWarnsWithoutSearching(t *testing.T) {
	searchCalls := 0
	retriever := NewDualRetriever(
		&recordingEmbedder{err: errors.New("Ollama unavailable")}, "bge-m3",
		memorySearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			searchCalls++
			return nil, nil
		}),
		documentSearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			searchCalls++
			return nil, nil
		}),
	)
	result, err := retriever.Retrieve(context.Background(), validDualRequest())
	if err != nil || len(result.Warnings) != 1 || searchCalls != 0 {
		t.Fatalf("Retrieve() result=%#v error=%v calls=%d", result, err, searchCalls)
	}
}

func TestDualRetrieverRejectsSearcherHitWithWrongOwnership(t *testing.T) {
	wrongUser := validMemoryHit()
	wrongUser.UserID = "u-002"
	retriever := NewDualRetriever(
		&recordingEmbedder{vectors: [][]float32{{0.1}}}, "bge-m3",
		memorySearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			return []Hit{wrongUser}, nil
		}),
		documentSearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			return []Hit{validDocumentHit()}, nil
		}),
	)
	_, err := retriever.Retrieve(context.Background(), validDualRequest())
	if err == nil || IsInfrastructureFailure(err) || !strings.Contains(err.Error(), "belongs to user") {
		t.Fatalf("Retrieve() error = %v, want hard ownership failure", err)
	}
}

func TestDualRetrieverSkipsDisabledSourceAndValidatesRequest(t *testing.T) {
	documentCalls := 0
	retriever := NewDualRetriever(
		&recordingEmbedder{vectors: [][]float32{{0.1}}}, "bge-m3",
		memorySearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			return []Hit{validMemoryHit()}, nil
		}),
		documentSearchFunc(func(context.Context, string, []float32, int) ([]Hit, error) {
			documentCalls++
			return nil, nil
		}),
	)
	req := validDualRequest()
	req.UseDocuments = false
	req.KnowledgeScope = ""
	req.DocumentLimit = 0
	if _, err := retriever.Retrieve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if documentCalls != 0 {
		t.Fatalf("disabled document calls = %d", documentCalls)
	}

	invalid := []DualRequest{
		{},
		{Query: "q", UseMemory: true, MemoryLimit: 1},
		{Query: "q", UserID: "u", UseMemory: true, MemoryLimit: 0},
		{Query: "q", UseDocuments: true, DocumentLimit: 1},
		{Query: "q", KnowledgeScope: "scope", UseDocuments: true, DocumentLimit: 0},
	}
	for index, request := range invalid {
		if _, err := retriever.Retrieve(context.Background(), request); err == nil {
			t.Fatalf("invalid request %d error = nil", index)
		}
	}
}

func validDualRequest() DualRequest {
	return DualRequest{
		Query: "项目使用什么语言？", UserID: "u-001", KnowledgeScope: "offline-rag-course",
		UseMemory: true, UseDocuments: true, MemoryLimit: 3, DocumentLimit: 3,
	}
}

func validMemoryHit() Hit {
	return Hit{Source: SourceMemory, ID: "memory:7", Content: "project_fact/language: Go", Score: 0.9, UserID: "u-001"}
}

func validDocumentHit() Hit {
	return Hit{Source: SourceDocument, ID: "document:7", Content: "Go course", Score: 0.8, KnowledgeScope: "offline-rag-course"}
}
