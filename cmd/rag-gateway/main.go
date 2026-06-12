package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"offline-rag-go-lab/internal/gateway"
)

func main() {
	app := gateway.NewApp(gateway.Config{
		LogDir:              filepath.Join("storage", "logs"),
		DocDir:              filepath.Join("storage", "docs"),
		RetrievalTopK:       5,
		ScoreThreshold:      0.1,
		PromptMaxChunks:     4,
		PromptMaxChars:      1200,
		ChatModel:           envOrDefault("OLLAMA_CHAT_MODEL", "mock-chat"),
		EmbeddingModel:      envOrDefault("OLLAMA_EMBED_MODEL", "mock-embedding"),
		KnowledgeCollection: "knowledge_chunks",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "offline-rag-go-lab"})
	})
	mux.HandleFunc("/debug/retrieval", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.DebugRetrieval(r.URL.Query().Get("question")))
	})
	mux.HandleFunc("/debug/prompt", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.PromptPreview(r.URL.Query().Get("question")))
	})
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req gateway.IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		resp, err := app.IngestText(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/debug/split", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req gateway.IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		resp, err := app.SplitPreview(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req gateway.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		resp, err := app.Chat(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	addr := ":18092"
	log.Printf("offline-rag-go-lab listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	raw, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
