package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/sessionsummary"
)

const (
	primaryMemoryUserID   = "memory-store-demo-user-20260712-a"
	secondaryMemoryUserID = "memory-qdrant-demo-other-user-20260712-a"
	secondarySessionID    = "memory-qdrant-demo-session-20260712-a"
	secondaryMessage      = "这个测试用户的项目也使用 Go。"
	demoCollection        = "offline_rag_memory_items_v1"
)

type demoConfig struct {
	MySQLDSN         string
	OllamaBaseURL    string
	EmbeddingModel   string
	QdrantBaseURL    string
	QdrantCollection string
}

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local MySQL, Ollama, and Qdrant config file")
	ensureCollection := flag.Bool("ensure-collection", false, "create or validate the dedicated Qdrant collection")
	flag.Parse()
	if !*ensureCollection {
		log.Fatal("--ensure-collection is required because this demo may create the dedicated collection")
	}

	config, err := loadDemoConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateDemoCollection(config.QdrantCollection); err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("mysql", config.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping MySQL: %v", err)
	}

	store := memoryitem.NewMySQLMemoryStore(db)
	primary, err := requiredActiveItem(ctx, store, primaryMemoryUserID, memoryitem.KindProjectFact, "implementation_language")
	if err != nil {
		log.Fatal(err)
	}
	secondary, secondaryAction, err := ensureSecondaryMemory(ctx, db, store)
	if err != nil {
		log.Fatal(err)
	}
	forgotten, found, err := store.Get(ctx, primaryMemoryUserID, memoryitem.KindPreference, "temporary_tool")
	if err != nil {
		log.Fatal(err)
	}
	if !found || forgotten.Status != memoryitem.StatusForgotten {
		log.Fatal("expected the lesson 22 temporary_tool item to exist with forgotten status")
	}

	embedder := memoryitem.NewHTTPOllamaEmbedder(config.OllamaBaseURL)
	items := []memoryitem.Item{primary, secondary}
	texts := make([]string, len(items))
	for index, item := range items {
		texts[index] = memoryEmbeddingText(item)
	}
	vectors, err := embedder.Embed(ctx, config.EmbeddingModel, texts)
	if err != nil {
		log.Fatal(err)
	}
	dimension := len(vectors[0])

	indexer := memoryitem.NewQdrantIndexer(config.QdrantBaseURL, config.QdrantCollection)
	if err := indexer.EnsureCollection(ctx, dimension); err != nil {
		log.Fatal(err)
	}
	for index, item := range items {
		if err := indexer.Upsert(ctx, item, vectors[index], config.EmbeddingModel); err != nil {
			log.Fatal(err)
		}
	}
	// A forgotten MySQL item must not remain retrievable from the derived index.
	if err := indexer.Delete(ctx, forgotten.ID); err != nil {
		log.Fatal(err)
	}

	queryVectors, err := embedder.Embed(ctx, config.EmbeddingModel, []string{"这个项目使用什么编程语言？"})
	if err != nil {
		log.Fatal(err)
	}
	if len(queryVectors[0]) != dimension {
		log.Fatalf("query vector dimension=%d, item dimension=%d", len(queryVectors[0]), dimension)
	}
	primaryResults, err := indexer.Search(ctx, primaryMemoryUserID, memoryitem.KindProjectFact, queryVectors[0], 5)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateTopResult(primaryResults, primaryMemoryUserID, primary.ID); err != nil {
		log.Fatal(err)
	}
	secondaryResults, err := indexer.Search(ctx, secondaryMemoryUserID, memoryitem.KindProjectFact, queryVectors[0], 5)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateTopResult(secondaryResults, secondaryMemoryUserID, secondary.ID); err != nil {
		log.Fatal(err)
	}
	forgottenResults, err := indexer.Search(ctx, primaryMemoryUserID, memoryitem.KindPreference, queryVectors[0], 5)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Embedding model: %s\n", config.EmbeddingModel)
	fmt.Printf("Vector dimension: %d\n", dimension)
	fmt.Printf("Collection: %s (Cosine)\n", config.QdrantCollection)
	fmt.Printf("Secondary MySQL fixture: action=%s item_id=%d\n", secondaryAction, secondary.ID)
	fmt.Printf("Upserted active items: %d\n", len(items))
	fmt.Printf("Search filter user_id=%s\n", primaryMemoryUserID)
	fmt.Printf("Top result: %s/%s score=%.6f\n", primaryResults[0].Kind, primaryResults[0].Key, primaryResults[0].Score)
	fmt.Printf("Cross-user point present: %t\n", containsOtherUser(primaryResults, primaryMemoryUserID))
	fmt.Printf("Secondary user top item: %d\n", secondaryResults[0].ItemID)
	fmt.Printf("Forgotten item present: %t\n", containsItem(forgottenResults, forgotten.ID))
}

func loadDemoConfig(path string) (demoConfig, error) {
	values, err := fileconfig.Load(path)
	if err != nil {
		return demoConfig{}, err
	}
	required := func(key string) (string, error) {
		return fileconfig.Required(values, key)
	}
	config := demoConfig{}
	if config.MySQLDSN, err = required("RECENT_CHAT_MYSQL_DSN"); err != nil {
		return demoConfig{}, err
	}
	if config.OllamaBaseURL, err = required("OLLAMA_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.EmbeddingModel, err = required("OLLAMA_EMBED_MODEL"); err != nil {
		return demoConfig{}, err
	}
	if config.QdrantBaseURL, err = required("QDRANT_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.QdrantCollection, err = required("QDRANT_MEMORY_COLLECTION"); err != nil {
		return demoConfig{}, err
	}
	return config, nil
}

func validateDemoCollection(collection string) error {
	if strings.TrimSpace(collection) != demoCollection {
		return fmt.Errorf("Qdrant demo collection must be %q", demoCollection)
	}
	return nil
}

func requiredActiveItem(ctx context.Context, store memoryitem.MemoryStore, userID string, kind memoryitem.Kind, key string) (memoryitem.Item, error) {
	item, found, err := store.Get(ctx, userID, kind, key)
	if err != nil {
		return memoryitem.Item{}, err
	}
	if !found || item.Status != memoryitem.StatusActive {
		return memoryitem.Item{}, fmt.Errorf("active memory %s/%s is required for user %s", kind, key, userID)
	}
	return item, nil
}

func ensureSecondaryMemory(ctx context.Context, db *sql.DB, store memoryitem.MemoryStore) (memoryitem.Item, memoryitem.Action, error) {
	source := sessionsummary.NewMySQLMessageSource(db)
	messages, err := source.ListAfter(secondarySessionID, secondaryMemoryUserID, 0)
	if err != nil {
		return memoryitem.Item{}, "", err
	}
	if len(messages) == 0 {
		messageStore := recentchat.NewMySQLMessageStore(db)
		if err := messageStore.Append(recentchat.Message{
			SessionID: secondarySessionID, UserID: secondaryMemoryUserID,
			Role: recentchat.RoleUser, Content: secondaryMessage, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return memoryitem.Item{}, "", err
		}
		messages, err = source.ListAfter(secondarySessionID, secondaryMemoryUserID, 0)
		if err != nil {
			return memoryitem.Item{}, "", err
		}
	}
	if len(messages) != 1 || strings.TrimSpace(messages[0].Role) != "user" || messages[0].Content != secondaryMessage {
		return memoryitem.Item{}, "", fmt.Errorf("secondary demo session must contain its one exact user fixture")
	}
	message := memoryitem.SourceMessage{
		ID: messages[0].ID, SessionID: secondarySessionID, UserID: secondaryMemoryUserID,
		Role: messages[0].Role, Content: messages[0].Content,
	}
	result, err := store.Apply(ctx, memoryitem.ApplyRequest{
		UserID: secondaryMemoryUserID, SessionID: secondarySessionID,
		Candidate: memoryitem.Candidate{
			Operation: memoryitem.OperationUpsert, Kind: memoryitem.KindProjectFact,
			Key: "implementation_language", Value: "Go", Confidence: 1,
			SourceMessageIDs: []int64{message.ID},
		},
		SourceMessages: []memoryitem.SourceMessage{message},
	})
	if err != nil {
		return memoryitem.Item{}, "", err
	}
	return result.Item, result.Decision.Action, nil
}

func memoryEmbeddingText(item memoryitem.Item) string {
	return fmt.Sprintf("%s/%s: %s", item.Kind, item.Key, item.Value)
}

func validateTopResult(results []memoryitem.SearchResult, userID string, expectedItemID int64) error {
	if len(results) == 0 {
		return fmt.Errorf("Qdrant returned no memory result for user %s", userID)
	}
	if results[0].UserID != userID {
		return fmt.Errorf("top result belongs to user %s, want %s", results[0].UserID, userID)
	}
	if results[0].ItemID != expectedItemID {
		return fmt.Errorf("top result item ID=%d, want %d", results[0].ItemID, expectedItemID)
	}
	return nil
}

func containsItem(results []memoryitem.SearchResult, itemID int64) bool {
	for _, result := range results {
		if result.ItemID == itemID {
			return true
		}
	}
	return false
}

func containsOtherUser(results []memoryitem.SearchResult, userID string) bool {
	for _, result := range results {
		if result.UserID != userID {
			return true
		}
	}
	return false
}
