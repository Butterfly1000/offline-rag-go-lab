package boss

import (
	"encoding/json" // 把日志记录序列化为 JSON 一行
	"os"            // OpenFile 追加写入
	"path/filepath" // 拼接日志文件路径
	"time"          // 时间戳与按天文件名

	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

// JSONLConversationLogger 每次 Chat 往按日期命名的 .jsonl 文件追加一行 JSON。
type JSONLConversationLogger struct {
	logDir         string // 日志目录（启动时已 MkdirAll）
	chatModel      string // 默认聊天模型名
	embeddingModel string // 默认 embedding 模型名
}

// NewJSONLConversationLogger 构造日志器。
func NewJSONLConversationLogger(logDir, chatModel, embeddingModel string) JSONLConversationLogger {
	return JSONLConversationLogger{
		logDir:         logDir,
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
	}
}

// AppendLog 写入一条对话记录；同一自然日共用一个文件。
func (l JSONLConversationLogger) AppendLog(req world.ChatRequest, resp world.ChatResponse, hits []world.RetrievalHit) error {
	record := map[string]any{
		"session_id":          req.SessionID,
		"user_id":             req.UserID,
		"question":            req.Question,
		"answer":              resp.Answer,
		"used_knowledge":      resp.UsedKnowledge,
		"retrieved_chunk_ids": shared.ChunkIDs(hits),
		"chat_model":          shared.ValueOrDefault(req.Model, l.chatModel), // 请求未指定 model 时用配置默认
		"embedding_model":     l.embeddingModel,
		"created_at":          time.Now().Format(time.RFC3339), // ISO8601 时间，如 2006-01-02T15:04:05+08:00
	}

	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}

	filename := time.Now().Format("2006-01-02") + ".jsonl" // Go 时间格式参考时间：2006-01-02
	path := filepath.Join(l.logDir, filename)
	// O_CREATE 不存在则创建；O_WRONLY 只写；O_APPEND 追加到文件末尾
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() // 函数返回前关闭文件句柄

	_, err = f.Write(append(raw, '\n')) // JSONL：每行一个 JSON 对象 + 换行
	return err
}
