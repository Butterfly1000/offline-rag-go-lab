package recentchat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOllamaClientGenerateJSONSendsSchemaAndLimit(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("request path = %q, want /api/chat", r.URL.Path)
		}
		var request OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "qwen:7b" || request.Stream {
			t.Fatalf("request model=%q stream=%t", request.Model, request.Stream)
		}
		if len(request.Messages) != 2 || request.Messages[0].Role != RoleSystem || request.Messages[1].Role != RoleUser {
			t.Fatalf("request messages = %#v", request.Messages)
		}
		if string(request.Format) != string(schema) {
			t.Fatalf("request format = %s, want %s", request.Format, schema)
		}
		if request.Options == nil || request.Options.NumPredict != 256 {
			t.Fatalf("request options = %#v", request.Options)
		}
		if request.Options.Temperature == nil || *request.Options.Temperature != 0 {
			t.Fatalf("request temperature = %#v, want explicit zero", request.Options.Temperature)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"{\"value\":\"Go\"}"}}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	got, err := client.GenerateJSON("qwen:7b", "system", "prompt", schema, 256)
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if string(got) != `{"value":"Go"}` {
		t.Fatalf("GenerateJSON() = %s", got)
	}
}

func TestHTTPOllamaClientGenerateJSONRejectsInvalidArgumentsAndEmptyOutput(t *testing.T) {
	client := NewHTTPOllamaClient("http://127.0.0.1:1")
	tests := []struct {
		name, model, system, prompt string
		schema                      json.RawMessage
		maxTokens                   int
	}{
		{name: "empty model", system: "system", prompt: "prompt", schema: json.RawMessage(`{"type":"object"}`), maxTokens: 1},
		{name: "empty system", model: "qwen:7b", prompt: "prompt", schema: json.RawMessage(`{"type":"object"}`), maxTokens: 1},
		{name: "empty prompt", model: "qwen:7b", system: "system", schema: json.RawMessage(`{"type":"object"}`), maxTokens: 1},
		{name: "empty schema", model: "qwen:7b", system: "system", prompt: "prompt", maxTokens: 1},
		{name: "invalid schema", model: "qwen:7b", system: "system", prompt: "prompt", schema: json.RawMessage(`{`), maxTokens: 1},
		{name: "invalid max tokens", model: "qwen:7b", system: "system", prompt: "prompt", schema: json.RawMessage(`{"type":"object"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := client.GenerateJSON(tt.model, tt.system, tt.prompt, tt.schema, tt.maxTokens); err == nil {
				t.Fatal("GenerateJSON() error = nil")
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"  "}}`))
	}))
	defer server.Close()
	if _, err := NewHTTPOllamaClient(server.URL).GenerateJSON(
		"qwen:7b", "system", "prompt", json.RawMessage(`{"type":"object"}`), 1,
	); err == nil {
		t.Fatal("GenerateJSON() empty output error = nil")
	}
}
