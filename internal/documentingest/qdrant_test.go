package documentingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQdrantIndexUpsertBatchesUsesWaitAndPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/collections/lab/points" || r.URL.Query().Get("wait") != "true" {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Points []VectorPoint `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Points) != 1 || body.Points[0].Payload.KnowledgeScope != "course" {
			t.Fatalf("body=%#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	index := NewQdrantIndex(server.URL)
	err := index.UpsertBatch(context.Background(), "lab", []VectorPoint{{ID: "id-1", Vector: []float32{0.1, 0.2}, Payload: VectorPayload{KnowledgeScope: "course", DocumentID: "intro", ChunkID: "chunk", Text: "text"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestQdrantIndexDeleteRejectsEmptySelector(t *testing.T) {
	err := NewQdrantIndex("http://127.0.0.1:1").DeletePoints(context.Background(), "lab", nil)
	if err == nil || !strings.Contains(err.Error(), "point") {
		t.Fatalf("error=%v", err)
	}
}
