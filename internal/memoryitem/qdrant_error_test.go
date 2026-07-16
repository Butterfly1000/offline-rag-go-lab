package memoryitem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQdrantDataErrorSurvivesWrapping(t *testing.T) {
	cause := errors.New("wrong user")
	err := fmt.Errorf("search failed: %w", &QdrantDataError{Err: cause})
	if !IsQdrantDataError(err) {
		t.Fatalf("IsQdrantDataError(%v) = false", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("wrapped error does not contain cause: %v", err)
	}
	if IsQdrantDataError(fmt.Errorf("HTTP 500")) {
		t.Fatal("ordinary infrastructure error classified as Qdrant data error")
	}
}

func TestQdrantIndexerClassifiesMalformedSearchResultsAsDataErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "point ID", body: `{"result":{"points":[{"id":"not-an-integer","score":0.9,"payload":{}}]}}`},
		{name: "wrong user", body: `{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-002","memory_item_id":7,"kind":"project_fact","memory_key":"language","value":"Go","version":1}}]}}`},
		{name: "invalid kind", body: `{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-001","memory_item_id":7,"kind":"bad","memory_key":"language","value":"Go","version":1}}]}}`},
		{name: "invalid key", body: `{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-001","memory_item_id":7,"kind":"project_fact","memory_key":"bad/key","value":"Go","version":1}}]}}`},
		{name: "invalid value", body: `{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-001","memory_item_id":7,"kind":"project_fact","memory_key":"language","value":"","version":1}}]}}`},
		{name: "invalid version", body: `{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-001","memory_item_id":7,"kind":"project_fact","memory_key":"language","value":"Go","version":0}}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newQdrantResponseServer(t, tt.body)
			defer server.Close()
			_, err := NewQdrantIndexer(server.URL, "memory").Search(
				context.Background(), "u-001", "", []float32{0.1}, 1,
			)
			if err == nil || !IsQdrantDataError(err) {
				t.Fatalf("Search() error = %v, want QdrantDataError", err)
			}
		})
	}
}

func TestQdrantPointToSearchResultRejectsNonFiniteScore(t *testing.T) {
	point := qdrantQueryPoint{
		ID: json.RawMessage(`7`), Score: math.NaN(),
		Payload: qdrantPayload{
			UserID: "u-001", MemoryItemID: 7, Kind: KindProjectFact,
			MemoryKey: "language", Value: "Go", Version: 1,
		},
	}
	_, err := qdrantPointToSearchResult(point, "u-001", "", 0)
	if err == nil || !IsQdrantDataError(err) {
		t.Fatalf("qdrantPointToSearchResult() error = %v, want QdrantDataError", err)
	}
}

func TestQdrantHTTPFailureIsNotDataError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	_, err := NewQdrantIndexer(server.URL, "memory").Search(context.Background(), "u-001", "", []float32{0.1}, 1)
	if err == nil || IsQdrantDataError(err) {
		t.Fatalf("Search() error = %v, want ordinary infrastructure error", err)
	}
}

func newQdrantResponseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}
