package boss

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

type JSONLConversationLogger struct {
	logDir         string
	chatModel      string
	embeddingModel string
}

func NewJSONLConversationLogger(logDir, chatModel, embeddingModel string) JSONLConversationLogger {
	return JSONLConversationLogger{
		logDir:         logDir,
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
	}
}

func (l JSONLConversationLogger) AppendLog(req world.ChatRequest, resp world.ChatResponse, hits []world.RetrievalHit) error {
	record := map[string]any{
		"session_id":          req.SessionID,
		"user_id":             req.UserID,
		"question":            req.Question,
		"answer":              resp.Answer,
		"used_knowledge":      resp.UsedKnowledge,
		"retrieved_chunk_ids": shared.ChunkIDs(hits),
		"chat_model":          shared.ValueOrDefault(req.Model, l.chatModel),
		"embedding_model":     l.embeddingModel,
		"created_at":          time.Now().Format(time.RFC3339),
	}

	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}

	filename := time.Now().Format("2006-01-02") + ".jsonl"
	path := filepath.Join(l.logDir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(raw, '\n'))
	return err
}
