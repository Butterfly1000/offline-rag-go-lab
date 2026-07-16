package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"offline-rag-go-lab/internal/contextretrieval"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
)

const (
	demoMemoryCollection   = "offline_rag_memory_items_v1"
	demoDocumentCollection = "offline_rag_document_chunks_v1"
	demoUserID             = "memory-store-demo-user-20260712-a"
	demoKnowledgeScope     = "offline-rag-course"
)

type demoConfig struct {
	OllamaBaseURL      string
	EmbeddingModel     string
	QdrantBaseURL      string
	MemoryCollection   string
	DocumentCollection string
}

type countingEmbedder struct {
	inner memoryitem.Embedder
	calls int
}

func (e *countingEmbedder) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	e.calls++
	return e.inner.Embed(ctx, model, texts)
}

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local Ollama and Qdrant config file")
	flag.Parse()
	config, err := loadDemoConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if config.MemoryCollection != demoMemoryCollection {
		log.Fatalf("memory collection must be %q", demoMemoryCollection)
	}
	if config.DocumentCollection != demoDocumentCollection {
		log.Fatalf("document collection must be %q", demoDocumentCollection)
	}

	embedder := &countingEmbedder{inner: memoryitem.NewHTTPOllamaEmbedder(config.OllamaBaseURL)}
	memory := contextretrieval.NewMemoryQdrantSearcher(
		memoryitem.NewQdrantIndexer(config.QdrantBaseURL, config.MemoryCollection),
	)
	documents := contextretrieval.NewDocumentQdrant(config.QdrantBaseURL, config.DocumentCollection)
	retriever := contextretrieval.NewDualRetriever(embedder, config.EmbeddingModel, memory, documents)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := retriever.Retrieve(ctx, contextretrieval.DualRequest{
		Query:  "这个项目使用什么语言，聊天历史如何按 token 预算处理？",
		UserID: demoUserID, KnowledgeScope: demoKnowledgeScope,
		UseMemory: true, UseDocuments: true, MemoryLimit: 5, DocumentLimit: 5,
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		log.Fatalf("standalone demo requires both sources healthy, warnings: %s", strings.Join(result.Warnings, "; "))
	}
	if len(result.MemoryHits) == 0 || len(result.DocumentHits) == 0 {
		log.Fatal("both memory and document retrieval must return at least one hit")
	}

	fmt.Printf("Query embeddings: %d\n", embedder.calls)
	fmt.Println("Memory hits:")
	for _, hit := range result.MemoryHits {
		fmt.Printf("- %s score=%.6f %s\n", hit.ID, hit.Score, hit.Content)
	}
	fmt.Println("Document hits:")
	for _, hit := range result.DocumentHits {
		fmt.Printf("- %s score=%.6f %s\n", hit.ID, hit.Score, hit.Title)
	}
	fmt.Printf("Retrieval warnings: %d\n", len(result.Warnings))
	fmt.Printf("Cross-user memory present: %t\n", containsOtherUser(result.MemoryHits, demoUserID))
	fmt.Printf("Cross-scope document present: %t\n", containsOtherScope(result.DocumentHits, demoKnowledgeScope))
}

func loadDemoConfig(path string) (demoConfig, error) {
	values, err := fileconfig.Load(path)
	if err != nil {
		return demoConfig{}, err
	}
	required := func(key string) (string, error) { return fileconfig.Required(values, key) }
	config := demoConfig{}
	if config.OllamaBaseURL, err = required("OLLAMA_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.EmbeddingModel, err = required("OLLAMA_EMBED_MODEL"); err != nil {
		return demoConfig{}, err
	}
	if config.QdrantBaseURL, err = required("QDRANT_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.MemoryCollection, err = required("QDRANT_MEMORY_COLLECTION"); err != nil {
		return demoConfig{}, err
	}
	if config.DocumentCollection, err = required("QDRANT_DOCUMENT_COLLECTION"); err != nil {
		return demoConfig{}, err
	}
	return config, nil
}

func containsOtherUser(hits []contextretrieval.Hit, userID string) bool {
	for _, hit := range hits {
		if hit.UserID != userID {
			return true
		}
	}
	return false
}

func containsOtherScope(hits []contextretrieval.Hit, scope string) bool {
	for _, hit := range hits {
		if hit.KnowledgeScope != scope {
			return true
		}
	}
	return false
}
