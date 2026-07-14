package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/sessionsummary"
)

const (
	demoSessionID = "memory-store-demo-20260712-a"
	demoUserID    = "memory-store-demo-user-20260712-a"
)

var demoMessageContents = []string{
	"这个项目使用 Go。",
	"这个项目仍然使用 Go。",
	"这个项目现在改用 Rust。",
	"这个项目重新使用 Go。",
	"临时工具使用 Vim。",
	"请忘掉临时工具偏好。",
}

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local MySQL config file")
	applySchemaFlag := flag.Bool("apply-schema", false, "apply the configured memory schema before the demo")
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping MySQL: %v", err)
	}
	if *applySchemaFlag {
		schemaPath := strings.TrimSpace(values["MEMORY_STORE_SCHEMA_PATH"])
		if schemaPath == "" {
			schemaPath = "sql/memory_items.sql"
		}
		if err := applySchema(ctx, db, schemaPath); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Applied schema: %s\n", schemaPath)
	}

	source := sessionsummary.NewMySQLMessageSource(db)
	messages, err := source.ListAfter(demoSessionID, demoUserID, 0)
	if err != nil {
		log.Fatal(err)
	}
	if len(messages) == 0 {
		if err := seedMessages(recentchat.NewMySQLMessageStore(db), demoSessionID, demoUserID); err != nil {
			log.Fatal(err)
		}
		messages, err = source.ListAfter(demoSessionID, demoUserID, 0)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Seeded source messages: %d\n", len(messages))
	}

	steps, err := buildDemoSteps(demoSessionID, demoUserID, messages)
	if err != nil {
		log.Fatal(err)
	}
	store := memoryitem.NewMySQLMemoryStore(db)
	complete, err := inspectDemoState(ctx, store, db)
	if err != nil {
		log.Fatal(err)
	}
	if complete {
		fmt.Println("Demo state already complete; no writes applied.")
		if err := printDemoSummary(ctx, store, db); err != nil {
			log.Fatal(err)
		}
		return
	}
	for _, step := range steps {
		result, err := store.Apply(ctx, step)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf(
			"%-6s %s/%s version=%d status=%s evidence_inserted=%d\n",
			result.Decision.Action,
			result.Item.Kind,
			result.Item.Key,
			result.Item.Version,
			result.Item.Status,
			result.EvidenceInserted,
		)
	}

	if err := printDemoSummary(ctx, store, db); err != nil {
		log.Fatal(err)
	}
}

func inspectDemoState(ctx context.Context, store memoryitem.MemoryStore, db *sql.DB) (bool, error) {
	primary, primaryFound, err := store.Get(ctx, demoUserID, memoryitem.KindProjectFact, "implementation_language")
	if err != nil {
		return false, err
	}
	temporary, temporaryFound, err := store.Get(ctx, demoUserID, memoryitem.KindPreference, "temporary_tool")
	if err != nil {
		return false, err
	}
	var evidenceCount int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM memory_item_evidence WHERE user_id = ?", demoUserID,
	).Scan(&evidenceCount); err != nil {
		return false, err
	}
	return classifyDemoState(primary, primaryFound, temporary, temporaryFound, evidenceCount)
}

// classifyDemoState prevents a rerun from replaying updates and increasing versions.
func classifyDemoState(primary memoryitem.Item, primaryFound bool, temporary memoryitem.Item, temporaryFound bool, evidenceCount int) (bool, error) {
	if !primaryFound && !temporaryFound && evidenceCount == 0 {
		return false, nil
	}
	primaryOK := primaryFound && primary.ID > 0 && primary.UserID == demoUserID &&
		primary.Kind == memoryitem.KindProjectFact && primary.Key == "implementation_language" &&
		primary.Value == "Go" && primary.Status == memoryitem.StatusActive && primary.Version == 3
	temporaryOK := temporaryFound && temporary.ID > 0 && temporary.UserID == demoUserID &&
		temporary.Kind == memoryitem.KindPreference && temporary.Key == "temporary_tool" &&
		temporary.Value == "Vim" && temporary.Status == memoryitem.StatusForgotten && temporary.Version == 2
	if primaryOK && temporaryOK && evidenceCount == len(demoMessageContents) {
		return true, nil
	}
	return false, fmt.Errorf("dedicated demo data is partial or differs from the expected terminal state")
}

func printDemoSummary(ctx context.Context, store memoryitem.MemoryStore, db *sql.DB) error {
	active, err := store.ListActive(ctx, demoUserID)
	if err != nil {
		return err
	}
	var evidenceCount int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM memory_item_evidence WHERE user_id = ?", demoUserID,
	).Scan(&evidenceCount); err != nil {
		return err
	}
	fmt.Printf("Active items: %d\n", len(active))
	fmt.Printf("Evidence rows: %d\n", evidenceCount)
	return nil
}

func applySchema(ctx context.Context, db *sql.DB, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read memory schema %s: %w", path, err)
	}
	for _, statement := range strings.Split(string(content), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply memory schema statement: %w", err)
		}
	}
	return nil
}

func seedMessages(store recentchat.MessageStore, sessionID, userID string) error {
	now := time.Now().UTC()
	for index, content := range demoMessageContents {
		if err := store.Append(recentchat.Message{
			SessionID: sessionID, UserID: userID, Role: recentchat.RoleUser,
			Content: content, CreatedAt: now.Add(time.Duration(index) * time.Microsecond),
		}); err != nil {
			return fmt.Errorf("seed source message %d: %w", index+1, err)
		}
	}
	return nil
}

func buildDemoSteps(sessionID, userID string, messages []sessionsummary.SourceMessage) ([]memoryitem.ApplyRequest, error) {
	if len(messages) != len(demoMessageContents) {
		return nil, fmt.Errorf("dedicated demo session must contain exactly %d messages, found %d", len(demoMessageContents), len(messages))
	}
	for index, source := range messages {
		if strings.ToLower(strings.TrimSpace(source.Role)) != string(recentchat.RoleUser) {
			return nil, fmt.Errorf("dedicated demo message %d role must be user", index+1)
		}
		if source.Content != demoMessageContents[index] {
			return nil, fmt.Errorf("dedicated demo message %d content differs from the expected fixture", index+1)
		}
	}
	types := []struct {
		operation memoryitem.Operation
		kind      memoryitem.Kind
		key       string
		value     string
	}{
		{memoryitem.OperationUpsert, memoryitem.KindProjectFact, "implementation_language", "Go"},
		{memoryitem.OperationUpsert, memoryitem.KindProjectFact, "implementation_language", "Go"},
		{memoryitem.OperationUpsert, memoryitem.KindProjectFact, "implementation_language", "Rust"},
		{memoryitem.OperationUpsert, memoryitem.KindProjectFact, "implementation_language", "Go"},
		{memoryitem.OperationUpsert, memoryitem.KindPreference, "temporary_tool", "Vim"},
		{memoryitem.OperationForget, memoryitem.KindPreference, "temporary_tool", ""},
	}
	steps := make([]memoryitem.ApplyRequest, 0, len(messages))
	for index, source := range messages {
		message := memoryitem.SourceMessage{
			ID: source.ID, SessionID: sessionID, UserID: userID,
			Role: source.Role, Content: source.Content,
		}
		definition := types[index]
		steps = append(steps, memoryitem.ApplyRequest{
			UserID: userID, SessionID: sessionID,
			Candidate: memoryitem.Candidate{
				Operation: definition.operation, Kind: definition.kind,
				Key: definition.key, Value: definition.value, Confidence: 1,
				SourceMessageIDs: []int64{source.ID},
			},
			SourceMessages: []memoryitem.SourceMessage{message},
		})
	}
	return steps, nil
}
