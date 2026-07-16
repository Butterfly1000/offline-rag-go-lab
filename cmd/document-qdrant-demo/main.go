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

const demoDocumentCollection = "offline_rag_document_chunks_v1"

type demoConfig struct {
	OllamaBaseURL    string
	EmbeddingModel   string
	QdrantBaseURL    string
	QdrantCollection string
}

var demoChunks = []contextretrieval.DocumentChunk{
	{
		KnowledgeScope: "offline-rag-course",
		DocumentID:     "recent-window-course",
		ChunkID:        "recent-window-course-001",
		Title:          "Recent Window",
		SourceRef:      "docs/teaching/recent-window-layer-01.md",
		Text:           "Recent Window 从 MySQL 按消息顺序读取当前会话最近的用户和助手消息。",
	},
	{
		KnowledgeScope: "offline-rag-course",
		DocumentID:     "token-budget-course",
		ChunkID:        "token-budget-course-001",
		Title:          "Token Budget",
		SourceRef:      "docs/teaching/recent-window-layer-02b-token-budget.md",
		Text:           "Token Budget 使用与模型匹配的 tokenizer 计算上下文占用，再按预算裁剪消息。",
	},
	{
		KnowledgeScope: "another-course",
		DocumentID:     "private-course",
		ChunkID:        "private-course-001",
		Title:          "Another Course",
		SourceRef:      "fixtures/another-course.md",
		Text:           "这段内容属于另一个知识范围，不能被 offline-rag-course 检索到。",
	},
}

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local Ollama and Qdrant config file")
	apply := flag.Bool("apply", false, "create or validate the dedicated collection and upsert fixed demo chunks")
	flag.Parse()
	if !*apply {
		log.Fatal("--apply is required because this demo writes fixed points to the dedicated document collection")
	}

	config, err := loadDemoConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if config.QdrantCollection != demoDocumentCollection {
		log.Fatalf("document Qdrant collection must be %q", demoDocumentCollection)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	embedder := memoryitem.NewHTTPOllamaEmbedder(config.OllamaBaseURL)

	texts := make([]string, len(demoChunks))
	for index, chunk := range demoChunks {
		texts[index] = chunk.Text
	}
	vectors, err := embedder.Embed(ctx, config.EmbeddingModel, texts)
	if err != nil {
		log.Fatal(err)
	}
	dimension := len(vectors[0])
	if dimension != 1024 {
		log.Fatalf("embedding dimension=%d, want 1024 for bge-m3", dimension)
	}

	store := contextretrieval.NewDocumentQdrant(config.QdrantBaseURL, config.QdrantCollection)
	if err := store.EnsureCollection(ctx, dimension); err != nil {
		log.Fatal(err)
	}
	for index, chunk := range demoChunks {
		if err := store.Upsert(ctx, chunk, vectors[index], config.EmbeddingModel); err != nil {
			log.Fatal(err)
		}
	}

	queryVectors, err := embedder.Embed(ctx, config.EmbeddingModel, []string{"聊天历史如何按 token 预算裁剪？"})
	if err != nil {
		log.Fatal(err)
	}
	primaryHits, err := store.Search(ctx, "offline-rag-course", queryVectors[0], 5)
	if err != nil {
		log.Fatal(err)
	}
	otherHits, err := store.Search(ctx, "another-course", queryVectors[0], 5)
	if err != nil {
		log.Fatal(err)
	}
	if len(primaryHits) == 0 || len(otherHits) == 0 {
		log.Fatal("each knowledge scope must return its own fixed fixture")
	}

	idsStable, err := demoPointIDsAreStable()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Embedding model: %s\n", config.EmbeddingModel)
	fmt.Printf("Vector dimension: %d\n", dimension)
	fmt.Printf("Collection: %s (Cosine)\n", config.QdrantCollection)
	fmt.Printf("Primary top result: %s score=%.6f\n", primaryHits[0].Title, primaryHits[0].Score)
	fmt.Printf("Other scope top result: %s score=%.6f\n", otherHits[0].Title, otherHits[0].Score)
	fmt.Printf("Cross-scope point present: %t\n", containsScope(primaryHits, "another-course"))
	fmt.Printf("Idempotent point IDs: %t\n", idsStable)
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
	if config.OllamaBaseURL, err = required("OLLAMA_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.EmbeddingModel, err = required("OLLAMA_EMBED_MODEL"); err != nil {
		return demoConfig{}, err
	}
	if config.QdrantBaseURL, err = required("QDRANT_BASE_URL"); err != nil {
		return demoConfig{}, err
	}
	if config.QdrantCollection, err = required("QDRANT_DOCUMENT_COLLECTION"); err != nil {
		return demoConfig{}, err
	}
	config.QdrantCollection = strings.TrimSpace(config.QdrantCollection)
	return config, nil
}

func containsScope(hits []contextretrieval.Hit, scope string) bool {
	for _, hit := range hits {
		if hit.KnowledgeScope == scope {
			return true
		}
	}
	return false
}

func demoPointIDsAreStable() (bool, error) {
	for _, chunk := range demoChunks {
		first, err := contextretrieval.DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
		if err != nil {
			return false, err
		}
		second, err := contextretrieval.DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
		if err != nil {
			return false, err
		}
		if first != second {
			return false, nil
		}
	}
	return true, nil
}
