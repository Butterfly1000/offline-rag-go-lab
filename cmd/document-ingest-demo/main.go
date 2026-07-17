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
	"offline-rag-go-lab/internal/documentingest"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local project config file")
	applySchema := flag.Bool("apply-schema", false, "apply the idempotent document schema before ingestion")
	collection := flag.String("collection", "", "configured isolated physical Qdrant collection")
	scope := flag.String("scope", "", "knowledge scope")
	documentID := flag.String("document-id", "", "logical document ID")
	format := flag.String("format", "", "document format: markdown or go")
	source := flag.String("source", "", "repository-relative source path")
	maxTokens := flag.Int("max-tokens", 160, "hard token ceiling per chunk")
	overlapLines := flag.Int("overlap-lines", 2, "line overlap for oversized structures")
	batchSize := flag.Int("batch-size", 16, "maximum texts in one Ollama/Qdrant batch")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	required := func(key string) string {
		value, err := fileconfig.Required(values, key)
		if err != nil {
			log.Fatal(err)
		}
		return value
	}
	allowed := map[string]bool{required("DOCUMENT_INGEST_COLLECTION_V1"): true, required("DOCUMENT_INGEST_COLLECTION_V2"): true}
	*collection = strings.TrimSpace(*collection)
	if !allowed[*collection] {
		log.Fatalf("--collection must equal DOCUMENT_INGEST_COLLECTION_V1 or DOCUMENT_INGEST_COLLECTION_V2")
	}
	content, err := os.ReadFile(*source)
	if err != nil {
		log.Fatalf("read source %s: %v", *source, err)
	}

	db, err := sql.Open("mysql", required("RECENT_CHAT_MYSQL_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping MySQL: %v", err)
	}
	if *applySchema {
		if err := applySQLFile(ctx, db, required("DOCUMENT_INGEST_SCHEMA_PATH")); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Applied schema: %s\n", required("DOCUMENT_INGEST_SCHEMA_PATH"))
	}
	counter, err := documentingest.NewQwenTokenCounter(required("RECENT_CHAT_TOKENIZER_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	service := documentingest.IngestionService{
		Store:    documentingest.NewMySQLManifestStore(db),
		Index:    documentingest.NewQdrantIndex(required("QDRANT_BASE_URL")),
		Embedder: memoryitem.NewHTTPOllamaEmbedder(required("OLLAMA_BASE_URL")),
		Counter:  counter,
	}
	result, err := service.Ingest(ctx, documentingest.IngestRequest{
		Document:      documentingest.Document{KnowledgeScope: *scope, DocumentID: *documentID, SourceRef: *source, Format: documentingest.DocumentFormat(*format), Content: content},
		Policy:        documentingest.ChunkPolicy{MaxTokens: *maxTokens, OverlapLines: *overlapLines},
		ParserVersion: "markdown-atx-go-ast-v1", TargetCollection: *collection,
		EmbeddingModel: required("OLLAMA_EMBED_MODEL"), BatchSize: *batchSize,
	})
	if err != nil {
		log.Fatal(err)
	}
	var manifestRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_chunk_manifests WHERE document_version_id = ?`, result.VersionID).Scan(&manifestRows); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Collection: %s\n", *collection)
	fmt.Printf("Version ID: %d\n", result.VersionID)
	fmt.Printf("Noop: %t\n", result.Noop)
	fmt.Printf("Chunks: %d\n", result.ChunkCount)
	fmt.Printf("Embed batches: %d\n", result.EmbedBatches)
	fmt.Printf("Upsert batches: %d\n", result.UpsertBatches)
	fmt.Printf("Manifest rows: %d\n", manifestRows)
}

func applySQLFile(ctx context.Context, db *sql.DB, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", path, err)
	}
	for _, statement := range strings.Split(string(content), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema statement: %w", err)
		}
	}
	return nil
}
