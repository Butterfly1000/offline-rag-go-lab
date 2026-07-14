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

func TestHTTPOllamaEmbedderSendsBatchAndReturnsVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/embed" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		var request struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "bge-m3" || len(request.Input) != 2 || request.Input[1] != "第二条" {
			t.Fatalf("request = %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"bge-m3","embeddings":[[0.1,0.2],[0.3,0.4]]}`))
	}))
	defer server.Close()

	got, err := NewHTTPOllamaEmbedder(server.URL).Embed(
		context.Background(), " bge-m3 ", []string{"第一条", "第二条"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[0]) != 2 || got[1][0] != float32(0.3) {
		t.Fatalf("vectors = %#v", got)
	}
}

func TestHTTPOllamaEmbedderRejectsInvalidArguments(t *testing.T) {
	client := NewHTTPOllamaEmbedder("http://127.0.0.1:1")
	tests := []struct {
		name  string
		model string
		texts []string
	}{
		{name: "empty model", texts: []string{"text"}},
		{name: "empty texts", model: "bge-m3"},
		{name: "blank text", model: "bge-m3", texts: []string{"text", "  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := client.Embed(context.Background(), tt.model, tt.texts); err == nil {
				t.Fatal("Embed() error = nil")
			}
		})
	}
}

func TestHTTPOllamaEmbedderRejectsInvalidResponseShape(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{name: "empty embeddings", response: `{"embeddings":[]}`},
		{name: "count mismatch", response: `{"embeddings":[[0.1,0.2]]}`},
		{name: "empty vector", response: `{"embeddings":[[0.1,0.2],[]]}`},
		{name: "different dimensions", response: `{"embeddings":[[0.1,0.2],[0.3]]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()
			if _, err := NewHTTPOllamaEmbedder(server.URL).Embed(
				context.Background(), "bge-m3", []string{"one", "two"},
			); err == nil {
				t.Fatal("Embed() error = nil")
			}
		})
	}
}

func TestHTTPOllamaEmbedderReturnsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("runner unavailable"))
	}))
	defer server.Close()

	_, err := NewHTTPOllamaEmbedder(server.URL).Embed(context.Background(), "bge-m3", []string{"text"})
	if err == nil || !strings.Contains(err.Error(), "status 502") || !strings.Contains(err.Error(), "runner unavailable") {
		t.Fatalf("Embed() error = %v", err)
	}
}

func TestValidateEmbeddingVectorsRejectsNonFiniteValues(t *testing.T) {
	for name, vectors := range map[string][][]float32{
		"nan":      {{float32(math.NaN())}},
		"positive": {{float32(math.Inf(1))}},
		"negative": {{float32(math.Inf(-1))}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateEmbeddingVectors(vectors, 1); err == nil {
				t.Fatal("validateEmbeddingVectors() error = nil")
			}
		})
	}
}

func TestHTTPOllamaEmbedderPropagatesContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewHTTPOllamaEmbedder(server.URL).Embed(ctx, "bge-m3", []string{"text"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Embed() error = %v", err)
	}
}
