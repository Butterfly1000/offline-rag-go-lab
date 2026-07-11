package recentchat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOllamaClientGenerateTextSendsSystemPromptAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("request path = %q, want /api/chat", r.URL.Path)
		}
		var request OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(request.Messages) != 2 || request.Messages[0].Role != RoleSystem || request.Messages[1].Role != RoleUser {
			t.Fatalf("request messages = %#v", request.Messages)
		}
		if request.Options == nil || request.Options.NumPredict != 512 {
			t.Fatalf("request options = %#v", request.Options)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"new summary"}}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	got, err := client.GenerateText("qwen:7b", "system", "prompt", 512)
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if got != "new summary" {
		t.Fatalf("GenerateText() = %q", got)
	}
}
