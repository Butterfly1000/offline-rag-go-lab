package recentchat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOllamaClientShowReturnsModelSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/show" {
			t.Fatalf("request = %s %s, want POST /api/show", r.Method, r.URL.Path)
		}

		var request OllamaShowRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "qwen:7b" {
			t.Fatalf("request model = %q, want qwen:7b", request.Model)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "template": "{{ .Prompt }}",
  "parameters": "stop <|im_end|>",
  "capabilities": ["completion"],
  "details": {
    "family": "qwen2",
    "parameter_size": "8B",
    "quantization_level": "Q4_0"
  },
  "model_info": {
    "general.architecture": "qwen2",
    "qwen2.context_length": 32768
  }
}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	summary, err := client.Show("qwen:7b")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}

	want := OllamaModelSummary{
		Model:             "qwen:7b",
		Family:            "qwen2",
		Architecture:      "qwen2",
		ParameterSize:     "8B",
		QuantizationLevel: "Q4_0",
		ContextLength:     32768,
		Template:          "{{ .Prompt }}",
		Parameters:        "stop <|im_end|>",
		Capabilities:      []string{"completion"},
	}
	if !equalModelSummary(summary, want) {
		t.Fatalf("Show() = %+v, want %+v", summary, want)
	}
}

func TestHTTPOllamaClientShowReturnsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	_, err := client.Show("missing")
	if err == nil {
		t.Fatal("Show() error = nil, want server error")
	}
}

func TestHTTPOllamaClientShowRejectsMissingContextMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "model_info": {
    "general.architecture": "qwen2"
  }
}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	_, err := client.Show("qwen:7b")
	if err == nil {
		t.Fatal("Show() error = nil, want missing context metadata error")
	}
}

func TestHTTPOllamaClientContextLengthUsesShowMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "model_info": {
    "general.architecture": "qwen2",
    "qwen2.context_length": 32768
  }
}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	got, err := client.ContextLength("qwen:7b")
	if err != nil {
		t.Fatalf("ContextLength() error = %v", err)
	}
	if got != 32768 {
		t.Fatalf("ContextLength() = %d, want 32768", got)
	}
}

func equalModelSummary(left, right OllamaModelSummary) bool {
	if left.Model != right.Model ||
		left.Family != right.Family ||
		left.Architecture != right.Architecture ||
		left.ParameterSize != right.ParameterSize ||
		left.QuantizationLevel != right.QuantizationLevel ||
		left.ContextLength != right.ContextLength ||
		left.Template != right.Template ||
		left.Parameters != right.Parameters ||
		len(left.Capabilities) != len(right.Capabilities) {
		return false
	}
	for i := range left.Capabilities {
		if left.Capabilities[i] != right.Capabilities[i] {
			return false
		}
	}
	return true
}
