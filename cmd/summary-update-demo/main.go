package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/sessionsummary"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local MySQL/Ollama/tokenizer config file")
	sessionID := flag.String("session-id", "summary-update-demo", "dedicated demo session ID")
	userID := flag.String("user-id", "summary-update-user", "dedicated demo user ID")
	model := flag.String("model", "qwen:7b", "Ollama model")
	seed := flag.Bool("seed", false, "insert six demo messages when the session is empty")
	recentKeep := flag.Int("recent-keep", 2, "number of newest unsummarized messages kept verbatim")
	minMessages := flag.Int("min-messages", 4, "unsummarized message trigger threshold")
	minTokens := flag.Int("min-tokens", 100000, "unsummarized token trigger threshold")
	maxOutputTokens := flag.Int("max-output-tokens", 256, "maximum generated summary tokens")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	dsn, err := fileconfig.Required(values, "RECENT_CHAT_MYSQL_DSN")
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping MySQL: %v", err)
	}

	messageSource := sessionsummary.NewMySQLMessageSource(db)
	allMessages, err := messageSource.ListAfter(*sessionID, *userID, 0)
	if err != nil {
		log.Fatal(err)
	}
	if *seed && len(allMessages) == 0 {
		if err := seedDemoMessages(recentchat.NewMySQLMessageStore(db), *sessionID, *userID); err != nil {
			log.Fatal(err)
		}
		allMessages, err = messageSource.ListAfter(*sessionID, *userID, 0)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Seeded messages: %d\n", len(allMessages))
	}

	store := sessionsummary.NewMySQLSummaryStore(db)
	current, exists, err := store.Get(*sessionID, *userID)
	if err != nil {
		log.Fatal(err)
	}
	watermark := int64(0)
	if exists {
		watermark = current.LastMessageID
	}
	unsummarized, err := messageSource.ListAfter(*sessionID, *userID, watermark)
	if err != nil {
		log.Fatal(err)
	}
	recentStartID, err := chooseRecentStartID(unsummarized, *recentKeep)
	if err != nil {
		log.Fatal(err)
	}

	tokenizerPath := values["RECENT_CHAT_TOKENIZER_PATH"]
	if tokenizerPath == "" {
		tokenizerPath = filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json")
	}
	rawCounter, err := tokenizerdemo.LoadCounter(tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}
	policy, err := sessionsummary.NewTriggerPolicy(*minMessages, *minTokens)
	if err != nil {
		log.Fatal(err)
	}
	ollamaURL := values["OLLAMA_BASE_URL"]
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}
	service := sessionsummary.NewUpdateService(
		store,
		messageSource,
		sessionsummary.NewFormattedMessageCounter(rawCounter, chatprompt.QwenFormatter{}),
		policy,
		sessionsummary.NewGenerator(recentchat.NewHTTPOllamaClient(ollamaURL)),
	)
	result, err := service.Update(sessionsummary.UpdateRequest{
		SessionID:       *sessionID,
		UserID:          *userID,
		Model:           *model,
		RecentStartID:   recentStartID,
		MaxOutputTokens: *maxOutputTokens,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Unsummarized IDs: %s\n", formatMessageIDs(result.Selection.Unsummarized))
	fmt.Printf("Evicted IDs: %s\n", formatMessageIDs(result.Selection.Evicted))
	fmt.Printf("Recent start ID: %d\n", recentStartID)
	fmt.Printf("Decision: %s\n", result.Decision.Reason)
	fmt.Printf("Updated: %t\n", result.Updated)
	fmt.Printf("Summary version: %d\n", result.Summary.Version)
	fmt.Printf("Summary watermark: %d\n", result.Summary.LastMessageID)
	fmt.Printf("Summary content: %s\n", result.Summary.Content)
}

func seedDemoMessages(store recentchat.MessageStore, sessionID, userID string) error {
	turns := []struct {
		role    recentchat.MessageRole
		content string
	}{
		{role: recentchat.RoleUser, content: "我叫小黄，示例代码使用 Go。"},
		{role: recentchat.RoleAssistant, content: "已完成 recent window 和 token 自动预算。"},
		{role: recentchat.RoleUser, content: "教学要贴近真实生产，不只做模拟。"},
		{role: recentchat.RoleAssistant, content: "MySQL summary store 已支持 version 乐观锁。"},
		{role: recentchat.RoleUser, content: "最近两条消息需要保留原文。"},
		{role: recentchat.RoleAssistant, content: "下一步把摘要接入 /chat。"},
	}
	now := time.Now().UTC()
	for i, turn := range turns {
		if err := store.Append(recentchat.Message{
			SessionID: sessionID,
			UserID:    userID,
			Role:      turn.role,
			Content:   turn.content,
			CreatedAt: now.Add(time.Duration(i) * time.Microsecond),
		}); err != nil {
			return fmt.Errorf("seed message %d: %w", i+1, err)
		}
	}
	return nil
}

func chooseRecentStartID(messages []sessionsummary.SourceMessage, keep int) (int64, error) {
	if keep < 0 {
		return 0, fmt.Errorf("recent keep must not be negative: %d", keep)
	}
	if keep == 0 || len(messages) == 0 {
		return 0, nil
	}
	start := len(messages) - keep
	if start < 0 {
		start = 0
	}
	return messages[start].ID, nil
}

func formatMessageIDs(messages []sessionsummary.SourceMessage) string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, strconv.FormatInt(message.ID, 10))
	}
	return strings.Join(ids, ",")
}
