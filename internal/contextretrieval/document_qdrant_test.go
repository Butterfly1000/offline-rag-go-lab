package contextretrieval

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

func TestDocumentQdrantEnsureCollectionCreatesCosineAndIndexes(t *testing.T) {
	var created bool
	var indexed []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/collections/document/items"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/collections/document/items"):
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
			created = true
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
			indexed = append(indexed, body.FieldName)
			_, _ = w.Write([]byte(`{"result":true,"status":"ok"}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := NewDocumentQdrant(server.URL, "document/items")
	if err := client.EnsureCollection(context.Background(), 1024); err != nil {
		t.Fatal(err)
	}
	if !created || len(indexed) != 2 || indexed[0] != "knowledge_scope" || indexed[1] != "document_id" {
		t.Fatalf("created=%t indexed=%#v", created, indexed)
	}
}

func TestDocumentQdrantEnsureCollectionValidatesExistingConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":{"config":{"params":{"vectors":{"size":384,"distance":"Cosine"}}}}}`))
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()

	err := NewDocumentQdrant(server.URL, "documents").EnsureCollection(context.Background(), 1024)
	if err == nil || !strings.Contains(err.Error(), "uses size=384") {
		t.Fatalf("EnsureCollection() error = %v", err)
	}
}

func TestDocumentQdrantUpsertUsesDeterministicIDAndPayload(t *testing.T) {
	chunk := validDocumentChunk()
	wantID, err := DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasSuffix(r.URL.Path, "/points") || r.URL.Query().Get("wait") != "true" {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		var body struct {
			Points []struct {
				ID      string         `json:"id"`
				Vector  []float32      `json:"vector"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Points) != 1 || body.Points[0].ID != wantID || len(body.Points[0].Vector) != 2 {
			t.Fatalf("points = %#v", body.Points)
		}
		payload := body.Points[0].Payload
		if payload["knowledge_scope"] != chunk.KnowledgeScope || payload["document_id"] != chunk.DocumentID || payload["chunk_id"] != chunk.ChunkID {
			t.Fatalf("identity payload = %#v", payload)
		}
		if payload["text"] != chunk.Text || payload["embedding_model"] != "bge-m3" || len(payload["content_hash"].(string)) != 64 {
			t.Fatalf("content payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"result":{"status":"completed"},"status":"ok"}`))
	}))
	defer server.Close()

	if err := NewDocumentQdrant(server.URL, "documents").Upsert(context.Background(), chunk, []float32{0.1, 0.2}, "bge-m3"); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentQdrantSearchFiltersScopeAndRevalidatesPayload(t *testing.T) {
	chunk := validDocumentChunk()
	pointID, err := DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		t.Fatal(err)
	}
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
		if len(body.Query) != 2 || body.Limit != 3 || !body.WithPayload || body.WithVector || len(body.Filter.Must) != 1 {
			t.Fatalf("query body = %#v", body)
		}
		if body.Filter.Must[0].Key != "knowledge_scope" || body.Filter.Must[0].Match.Value != chunk.KnowledgeScope {
			t.Fatalf("filter = %#v", body.Filter.Must)
		}
		response := map[string]any{"result": map[string]any{"points": []any{map[string]any{
			"id": pointID, "score": 0.88, "payload": map[string]any{
				"knowledge_scope": chunk.KnowledgeScope, "document_id": chunk.DocumentID,
				"chunk_id": chunk.ChunkID, "title": chunk.Title, "source_ref": chunk.SourceRef,
				"text": chunk.Text, "content_hash": documentContentHash(chunk.Text), "embedding_model": "bge-m3",
			},
		}}}}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	hits, err := NewDocumentQdrant(server.URL, "documents").Search(context.Background(), chunk.KnowledgeScope, []float32{0.1, 0.2}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Source != SourceDocument || hits[0].ID != "document:"+pointID || hits[0].KnowledgeScope != chunk.KnowledgeScope {
		t.Fatalf("hits = %#v", hits)
	}
	if hits[0].Metadata["chunk_id"] != chunk.ChunkID || hits[0].Title != chunk.Title {
		t.Fatalf("hit payload = %#v", hits[0])
	}
}

func TestDocumentQdrantRejectsCrossScopeAndMalformedPayload(t *testing.T) {
	chunk := validDocumentChunk()
	pointID, _ := DeterministicDocumentPointID("other-course", chunk.ChunkID)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"points":[{"id":"` + pointID + `","score":0.9,"payload":{"knowledge_scope":"other-course","document_id":"project-overview","chunk_id":"chunk-001","text":"Go","content_hash":"x","embedding_model":"bge-m3"}}]}}`))
	}))
	defer server.Close()

	_, err := NewDocumentQdrant(server.URL, "documents").Search(context.Background(), chunk.KnowledgeScope, []float32{0.1}, 1)
	if err == nil || IsInfrastructureFailure(err) || !strings.Contains(err.Error(), "belongs to knowledge_scope") {
		t.Fatalf("cross-scope error = %v", err)
	}
}

func TestDocumentPointToHitRejectsMalformedIdentityAndContent(t *testing.T) {
	chunk := validDocumentChunk()
	pointID, err := DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		t.Fatal(err)
	}
	valid := documentQueryPoint{
		ID: pointID, Score: 0.9,
		Payload: documentPayload{
			KnowledgeScope: chunk.KnowledgeScope, DocumentID: chunk.DocumentID,
			ChunkID: chunk.ChunkID, Text: chunk.Text,
			ContentHash: documentContentHash(chunk.Text), EmbeddingModel: "bge-m3",
		},
	}
	tests := []struct {
		name string
		edit func(*documentQueryPoint)
		want string
	}{
		{name: "point ID", edit: func(p *documentQueryPoint) { p.ID = "wrong" }, want: "does not match payload identity"},
		{name: "empty text", edit: func(p *documentQueryPoint) { p.Payload.Text = "" }, want: "text is required"},
		{name: "content hash", edit: func(p *documentQueryPoint) { p.Payload.ContentHash = "wrong" }, want: "content_hash does not match"},
		{name: "embedding model", edit: func(p *documentQueryPoint) { p.Payload.EmbeddingModel = "" }, want: "embedding_model is required"},
		{name: "score", edit: func(p *documentQueryPoint) { p.Score = math.NaN() }, want: "score must be finite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			point := valid
			tt.edit(&point)
			_, err := documentPointToHit(point, chunk.KnowledgeScope)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("documentPointToHit() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDocumentQdrantClassifiesHTTPAndContextAsInfrastructure(t *testing.T) {
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 10000)))
	}))
	defer errorServer.Close()
	_, err := NewDocumentQdrant(errorServer.URL, "documents").Search(context.Background(), "scope", []float32{0.1}, 1)
	if err == nil || !IsInfrastructureFailure(err) || len(err.Error()) > 2600 || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("HTTP error = %v", err)
	}

	cancelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer cancelServer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewDocumentQdrant(cancelServer.URL, "documents").Search(ctx, "scope", []float32{0.1}, 1)
	if !errors.Is(err, context.Canceled) || !IsInfrastructureFailure(err) {
		t.Fatalf("context error = %v", err)
	}
}

func TestDocumentQdrantRejectsInvalidInputs(t *testing.T) {
	client := NewDocumentQdrant("http://127.0.0.1:1", "documents")
	if err := client.EnsureCollection(context.Background(), 0); err == nil {
		t.Fatal("zero vector size error = nil")
	}
	if err := client.Upsert(context.Background(), validDocumentChunk(), []float32{float32(math.NaN())}, "bge-m3"); err == nil {
		t.Fatal("NaN vector error = nil")
	}
	if _, err := client.Search(context.Background(), "", []float32{0.1}, 1); err == nil {
		t.Fatal("empty scope error = nil")
	}
	if _, err := client.Search(context.Background(), "scope", []float32{0.1}, 0); err == nil {
		t.Fatal("zero limit error = nil")
	}
}

func validDocumentChunk() DocumentChunk {
	return DocumentChunk{
		KnowledgeScope: "offline-rag-course", DocumentID: "project-overview", ChunkID: "chunk-001",
		Title: "项目说明", SourceRef: "docs/project.md", Text: "这个项目使用 Go。",
	}
}
