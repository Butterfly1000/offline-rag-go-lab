package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/sessionsummary"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local config file containing RECENT_CHAT_MYSQL_DSN")
	applySchema := flag.Bool("apply-schema", false, "apply CREATE TABLE IF NOT EXISTS before saving")
	schemaPath := flag.String("schema", "sql/session_summaries.sql", "session summary schema file")
	sessionID := flag.String("session-id", "summary-store-demo", "dedicated demo session ID")
	userID := flag.String("user-id", "summary-store-user", "dedicated demo user ID")
	content := flag.String("content", "用户偏好真实落地，代码示例使用 Go。", "summary content to save")
	watermark := flag.Int64("watermark", 20, "last message ID covered by the summary")
	flag.Parse()

	// Read the DSN from an ignored local file so credentials do not depend on
	// process environment variables or appear in shell history.
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
	if *applySchema {
		if err := executeSchema(ctx, db, *schemaPath); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Applied schema: %s\n", *schemaPath)
	}

	store := sessionsummary.NewMySQLSummaryStore(db)
	current, exists, err := store.Get(*sessionID, *userID)
	if err != nil {
		log.Fatal(err)
	}
	expectedVersion := int64(0)
	if exists {
		expectedVersion = current.Version
	}

	saved, err := store.Save(sessionsummary.SessionSummary{
		SessionID:     *sessionID,
		UserID:        *userID,
		Content:       *content,
		LastMessageID: *watermark,
	}, expectedVersion)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Existed before save: %v\n", exists)
	fmt.Printf("Expected version: %d\n", expectedVersion)
	fmt.Printf("Saved version: %d\n", saved.Version)
	fmt.Printf("Saved watermark: %d\n", saved.LastMessageID)
	fmt.Printf("Saved content: %s\n", saved.Content)
}

type schemaExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func executeSchema(ctx context.Context, db schemaExecutor, path string) error {
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("execute schema %s: %w", path, err)
	}
	return nil
}
