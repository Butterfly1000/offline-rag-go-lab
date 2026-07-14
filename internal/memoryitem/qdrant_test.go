package memoryitem

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQdrantIndexerEnsureCollectionCreatesCosineCollectionAndIndexes(t *testing.T) {
	var collectionCreated bool
	var indexedFields []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.EscapedPath(), "memory%2Fitems") {
			t.Fatalf("escaped path = %q", r.URL.EscapedPath())
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/memory/items"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":{"error":"not found"}}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/collections/memory/items"):
			var body struct {
				Vectors struct {
					Size     int    `json:"size"`
					Distance string `json:"distance"`
				} `json:"vectors"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Vectors.Size != 1024 || body.Vectors.Distance != "Cosine" {
				t.Fatalf("create body = %#v", body)
			}
			collectionCreated = true
			_, _ = w.Write([]byte(`{"result":true,"status":"ok"}`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/index"):
			if r.URL.Query().Get("wait") != "true" {
				t.Fatalf("index wait = %q", r.URL.Query().Get("wait"))
			}
			var body struct {
				FieldName   string `json:"field_name"`
				FieldSchema string `json:"field_schema"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.FieldSchema != "keyword" {
				t.Fatalf("field schema = %q", body.FieldSchema)
			}
			indexedFields = append(indexedFields, body.FieldName)
			_, _ = w.Write([]byte(`{"result":{"status":"completed"},"status":"ok"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	indexer := NewQdrantIndexer(server.URL, "memory/items")
	if err := indexer.EnsureCollection(context.Background(), 1024); err != nil {
		t.Fatal(err)
	}
	if !collectionCreated || len(indexedFields) != 2 || indexedFields[0] != "user_id" || indexedFields[1] != "kind" {
		t.Fatalf("created=%t fields=%#v", collectionCreated, indexedFields)
	}
}

func TestQdrantIndexerEnsureCollectionValidatesExistingConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name      string
		size      int
		distance  string
		wantError bool
	}{
		{name: "matching", size: 1024, distance: "Cosine"},
		{name: "wrong size", size: 384, distance: "Cosine", wantError: true},
		{name: "wrong distance", size: 1024, distance: "Dot", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			indexCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(existingCollectionResponse(tt.size, tt.distance)))
					return
				}
				if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/index") {
					indexCalls++
					_, _ = w.Write([]byte(`{"result":true,"status":"ok"}`))
					return
				}
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			}))
			defer server.Close()

			err := NewQdrantIndexer(server.URL, "memory").EnsureCollection(context.Background(), 1024)
			if (err != nil) != tt.wantError {
				t.Fatalf("EnsureCollection() error = %v", err)
			}
			if tt.wantError && indexCalls != 0 {
				t.Fatalf("mismatched collection index calls = %d", indexCalls)
			}
			if !tt.wantError && indexCalls != 2 {
				t.Fatalf("matching collection index calls = %d", indexCalls)
			}
		})
	}
}

func TestQdrantIndexerUpsertUsesItemIDAndVersionedPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasSuffix(r.URL.Path, "/points") || r.URL.Query().Get("wait") != "true" {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Points []struct {
				ID      int64          `json:"id"`
				Vector  []float32      `json:"vector"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Points) != 1 || body.Points[0].ID != 7 || len(body.Points[0].Vector) != 2 {
			t.Fatalf("points = %#v", body.Points)
		}
		payload := body.Points[0].Payload
		if payload["user_id"] != "u-001" || payload["memory_item_id"] != float64(7) || payload["version"] != float64(3) || payload["embedding_model"] != "bge-m3" {
			t.Fatalf("payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"result":{"status":"completed"},"status":"ok"}`))
	}))
	defer server.Close()

	item := Item{ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language", Value: "Go", Status: StatusActive, Version: 3}
	if err := NewQdrantIndexer(server.URL, "memory").Upsert(context.Background(), item, []float32{0.1, 0.2}, "bge-m3"); err != nil {
		t.Fatal(err)
	}
}

func TestQdrantIndexerSearchAlwaysFiltersUserAndOptionallyKind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/points/query") {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Query  []float32 `json:"query"`
			Filter struct {
				Must []struct {
					Key   string `json:"key"`
					Match struct {
						Value string `json:"value"`
					} `json:"match"`
				} `json:"must"`
			} `json:"filter"`
			Limit       int  `json:"limit"`
			WithPayload bool `json:"with_payload"`
			WithVector  bool `json:"with_vector"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Query) != 2 || body.Limit != 3 || !body.WithPayload || body.WithVector || len(body.Filter.Must) != 2 {
			t.Fatalf("query body = %#v", body)
		}
		if body.Filter.Must[0].Key != "user_id" || body.Filter.Must[0].Match.Value != "u-001" || body.Filter.Must[1].Key != "kind" {
			t.Fatalf("filters = %#v", body.Filter.Must)
		}
		_, _ = w.Write([]byte(`{"result":{"points":[{"id":7,"score":0.91,"payload":{"user_id":"u-001","memory_item_id":7,"kind":"project_fact","memory_key":"implementation_language","value":"Go","version":3,"embedding_model":"bge-m3"}}]},"status":"ok"}`))
	}))
	defer server.Close()

	results, err := NewQdrantIndexer(server.URL, "memory").Search(
		context.Background(), "u-001", KindProjectFact, []float32{0.1, 0.2}, 3,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ItemID != 7 || results[0].Score != 0.91 || results[0].Value != "Go" || results[0].Version != 3 {
		t.Fatalf("results = %#v", results)
	}
}

func TestQdrantIndexerDeleteUsesWaitAndPointSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/points/delete") || r.URL.Query().Get("wait") != "true" {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Points []int64 `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Points) != 1 || body.Points[0] != 9 {
			t.Fatalf("points = %#v", body.Points)
		}
		_, _ = w.Write([]byte(`{"result":{"status":"completed"},"status":"ok"}`))
	}))
	defer server.Close()

	if err := NewQdrantIndexer(server.URL, "memory").Delete(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
}

func TestQdrantIndexerRejectsInvalidInputsAndCrossUserResults(t *testing.T) {
	indexer := NewQdrantIndexer("http://127.0.0.1:1", "memory")
	item := Item{ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language", Value: "Go", Status: StatusActive, Version: 3}
	if err := indexer.EnsureCollection(context.Background(), 0); err == nil {
		t.Fatal("EnsureCollection() error = nil")
	}
	item.Status = StatusForgotten
	if err := indexer.Upsert(context.Background(), item, []float32{0.1}, "bge-m3"); err == nil {
		t.Fatal("Upsert() forgotten error = nil")
	}
	if err := indexer.Delete(context.Background(), 0); err == nil {
		t.Fatal("Delete() error = nil")
	}
	if _, err := indexer.Search(context.Background(), "", "", []float32{0.1}, 1); err == nil {
		t.Fatal("Search() empty user error = nil")
	}
	if _, err := indexer.Search(context.Background(), "u-001", "", []float32{float32(math.NaN())}, 1); err == nil {
		t.Fatal("Search() NaN error = nil")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"points":[{"id":7,"score":0.9,"payload":{"user_id":"u-002","memory_item_id":7,"kind":"project_fact","memory_key":"implementation_language","value":"Go","version":3,"embedding_model":"bge-m3"}}]}}`))
	}))
	defer server.Close()
	if _, err := NewQdrantIndexer(server.URL, "memory").Search(context.Background(), "u-001", "", []float32{0.1}, 1); err == nil {
		t.Fatal("Search() cross-user result error = nil")
	}
}

func TestQdrantIndexerTruncatesErrorBodyAndPropagatesContext(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 10000)))
	}))
	defer errorServer.Close()
	err := NewQdrantIndexer(errorServer.URL, "memory").Delete(context.Background(), 7)
	if err == nil || len(err.Error()) > 2500 || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("Delete() error length=%d error=%v", len(err.Error()), err)
	}

	cancelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer cancelServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewQdrantIndexer(cancelServer.URL, "memory").Search(ctx, "u-001", "", []float32{0.1}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search() error = %v", err)
	}
}

func existingCollectionResponse(size int, distance string) string {
	response := map[string]any{
		"result": map[string]any{
			"config": map[string]any{
				"params": map[string]any{
					"vectors": map[string]any{"size": size, "distance": distance},
				},
			},
		},
		"status": "ok",
	}
	encoded, _ := json.Marshal(response)
	return string(encoded)
}
