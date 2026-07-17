package documentingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQdrantAliasSwitchUsesOneAtomicActionRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/collections/aliases" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Actions []map[string]map[string]string `json:"actions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Actions) != 2 || body.Actions[0]["delete_alias"]["alias_name"] != "active" || body.Actions[1]["create_alias"]["collection_name"] != "offline_rag_document_ingestion_lab_v2" {
			t.Fatalf("actions=%#v", body.Actions)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	index := NewQdrantIndex(server.URL)
	if err := index.SwitchAlias(context.Background(), "active", "offline_rag_document_ingestion_lab_v1", "offline_rag_document_ingestion_lab_v2"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d, want 1", requests)
	}
}

func TestQdrantAliasResolveUsesGlobalAliasList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/aliases" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"aliases":[{"alias_name":"active","collection_name":"offline_rag_document_ingestion_lab_v1"}]}}`))
	}))
	defer server.Close()
	target, err := NewQdrantIndex(server.URL).ResolveAlias(context.Background(), "active")
	if err != nil {
		t.Fatal(err)
	}
	if target != "offline_rag_document_ingestion_lab_v1" {
		t.Fatalf("target=%q", target)
	}
}
