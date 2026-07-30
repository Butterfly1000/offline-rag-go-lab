package retrievalquality

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestQdrantEnsureCollectionCreatesNamedDenseSparseAndIndexes(t *testing.T) {
	var create map[string]any
	var indexed []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/offline_rag_retrieval_quality_lab_v1":
			http.Error(w, `{"status":"not found"}`, http.StatusNotFound)
		case r.Method == http.MethodPut && r.URL.Path == "/collections/offline_rag_retrieval_quality_lab_v1":
			decodeRequestJSON(t, r, &create)
			writeJSON(t, w, map[string]any{"result": true})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/index"):
			var body struct {
				FieldName string `json:"field_name"`
			}
			decodeRequestJSON(t, r, &body)
			indexed = append(indexed, body.FieldName)
			writeJSON(t, w, map[string]any{"result": true})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, err := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureCollection(context.Background(), 1024); err != nil {
		t.Fatal(err)
	}
	vectors := create["vectors"].(map[string]any)
	dense := vectors["dense"].(map[string]any)
	if dense["size"] != float64(1024) || dense["distance"] != "Cosine" {
		t.Fatalf("dense config=%v", dense)
	}
	sparse := create["sparse_vectors"].(map[string]any)["sparse"].(map[string]any)
	if sparse["modifier"] != "idf" {
		t.Fatalf("sparse config=%v", sparse)
	}
	if want := []string{"knowledge_scope", "document_id", "chunk_id"}; !reflect.DeepEqual(indexed, want) {
		t.Fatalf("indexed=%v, want %v", indexed, want)
	}
}

func TestQdrantRejectsNonContractDimensionBeforeHTTP(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	client, err := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureCollection(context.Background(), 2); err == nil {
		t.Fatal("non-contract dimension must fail")
	}
	if requests != 0 {
		t.Fatalf("requests=%d, want zero", requests)
	}
}

func TestQdrantRejectsRemoteBaseURL(t *testing.T) {
	if _, err := NewQdrant(
		"https://qdrant.example.com", RetrievalQualityCollection, RetrievalQualityAlias,
	); err == nil {
		t.Fatal("retrieval-quality lab must reject remote Qdrant")
	}
}

func TestQdrantUpsertUsesNamedVectorsAndStableIdentity(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || !strings.HasSuffix(r.URL.Path, "/points") {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		decodeRequestJSON(t, r, &body)
		writeJSON(t, w, map[string]any{"result": map[string]any{"status": "completed"}})
	}))
	defer server.Close()
	client, _ := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	chunk := Chunk{
		KnowledgeScope: "scope-a", DocumentID: "doc", ChunkID: "chunk", Title: "title",
		HeadingPath: "heading", SourceRef: "docs/a.md", StructureKind: "markdown",
		Content: "body", ContentHash: sha256Hex([]byte("body")),
	}
	err := client.Upsert(context.Background(), IndexedChunk{
		Chunk: chunk, Dense: denseVectorFixture(),
		Sparse:         SparseVector{Indices: []uint32{3, 8}, Values: []float32{1.5, 2}},
		EmbeddingModel: "bge-m3", EncoderID: "qwen2:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	points := body["points"].([]any)
	point := points[0].(map[string]any)
	if point["id"] != StablePointID("scope-a", "chunk") {
		t.Fatalf("point id=%v", point["id"])
	}
	vector := point["vector"].(map[string]any)
	if _, ok := vector["dense"]; !ok {
		t.Fatalf("missing dense vector: %v", vector)
	}
	if _, ok := vector["sparse"]; !ok {
		t.Fatalf("missing sparse vector: %v", vector)
	}
}

func TestQdrantDenseAndSparseQueriesAlwaysSendScopeFilter(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		decodeRequestJSON(t, r, &body)
		requests = append(requests, body)
		writeJSON(t, w, qdrantQueryFixture("scope-a", "doc", "chunk", "body", "bge-m3", "qwen2:test"))
	}))
	defer server.Close()
	client, _ := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	if _, err := client.QueryDense(context.Background(), "scope-a", denseVectorFixture(), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := client.QuerySparse(context.Background(), "scope-a", SparseVector{Indices: []uint32{7}, Values: []float32{1}}, 10); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0]["using"] != "dense" || requests[1]["using"] != "sparse" {
		t.Fatalf("query using values=%v", requests)
	}
	for _, request := range requests {
		filter := request["filter"].(map[string]any)
		must := filter["must"].([]any)[0].(map[string]any)
		if must["key"] != "knowledge_scope" || must["match"].(map[string]any)["value"] != "scope-a" {
			t.Fatalf("missing scope filter: %v", request)
		}
	}
}

func denseVectorFixture() []float32 {
	result := make([]float32, RetrievalQualityVectorSize)
	result[0] = 1
	return result
}

func TestQdrantRejectsCrossScopeAndEncoderMismatch(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		want     string
	}{
		{name: "cross scope", response: qdrantQueryFixture("scope-b", "doc", "chunk", "body", "bge-m3", "qwen2:test"), want: "scope-b"},
		{name: "encoder mismatch", response: qdrantQueryFixture("scope-a", "doc", "chunk", "body", "bge-m3", "wrong"), want: "encoder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.response)
			}))
			defer server.Close()
			client, _ := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
			client.SetExpectedIdentity("bge-m3", "qwen2:test")
			_, err := client.QuerySparse(context.Background(), "scope-a", SparseVector{Indices: []uint32{7}, Values: []float32{1}}, 10)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
}

func TestQdrantActivateAddsOnlyFixedAlias(t *testing.T) {
	var action map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			writeJSON(t, w, map[string]any{"result": map[string]any{"aliases": []any{}}})
		case r.Method == http.MethodPost && r.URL.Path == "/collections/aliases":
			var body struct {
				Actions []map[string]any `json:"actions"`
			}
			decodeRequestJSON(t, r, &body)
			action = body.Actions[0]
			writeJSON(t, w, map[string]any{"result": true})
		default:
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, _ := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	if err := client.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	create := action["create_alias"].(map[string]any)
	if create["collection_name"] != RetrievalQualityCollection || create["alias_name"] != RetrievalQualityAlias {
		t.Fatalf("alias action=%v", action)
	}
}

func TestQdrantInspectValidatesSchemaAndPointCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"result": map[string]any{
			"status": "green", "points_count": 24,
			"config": map[string]any{"params": map[string]any{
				"vectors":        map[string]any{"dense": map[string]any{"size": 1024, "distance": "Cosine"}},
				"sparse_vectors": map[string]any{"sparse": map[string]any{"modifier": "idf"}},
			}},
		}})
	}))
	defer server.Close()
	client, _ := NewQdrant(server.URL, RetrievalQualityCollection, RetrievalQualityAlias)
	state, err := client.Inspect(context.Background(), 1024, 24)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "green" || state.PointCount != 24 {
		t.Fatalf("state=%+v", state)
	}
}

func decodeRequestJSON(t *testing.T, r *http.Request, target any) {
	t.Helper()
	content, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("decode request %s: %v\n%s", r.URL.Path, err, content)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func qdrantQueryFixture(scope, documentID, chunkID, content, model, encoder string) map[string]any {
	return map[string]any{"result": map[string]any{"points": []any{map[string]any{
		"id": StablePointID(scope, chunkID), "score": 0.9,
		"payload": map[string]any{
			"knowledge_scope": scope, "document_id": documentID, "chunk_id": chunkID,
			"title": "title", "heading_path": "heading", "source_ref": "docs/a.md",
			"structure_kind": "markdown", "content": content,
			"content_hash": sha256Hex([]byte(content)), "embedding_model": model,
			"encoder_id": encoder,
		},
	}}}}
}
