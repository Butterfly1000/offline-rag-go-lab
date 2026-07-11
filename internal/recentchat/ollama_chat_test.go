package recentchat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOllamaClientChatSendsNumPredictOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request OllamaChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Options == nil || request.Options.NumPredict != 2048 {
			t.Fatalf("request options = %#v, want num_predict 2048", request.Options)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"answer"}}`))
	}))
	defer server.Close()

	client := NewHTTPOllamaClient(server.URL)
	_, err := client.Chat(OllamaChatRequest{
		Model:    "qwen:7b",
		Messages: []OllamaMessage{{Role: RoleUser, Content: "hello"}},
		Options:  &OllamaChatOptions{NumPredict: 2048},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}
