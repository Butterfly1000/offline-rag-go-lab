package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"offline-rag-go-lab/internal/contextretrieval"
	"offline-rag-go-lab/internal/documentingest"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
)

type scopedDocumentSearcher struct {
	client *contextretrieval.DocumentQdrant
}

func (s scopedDocumentSearcher) Search(ctx context.Context, scope string, vector []float32, limit int) ([]documentingest.EvaluationHit, error) {
	hits, err := s.client.Search(ctx, scope, vector, limit)
	if err != nil {
		return nil, err
	}
	result := make([]documentingest.EvaluationHit, len(hits))
	for i, hit := range hits {
		result[i] = documentingest.EvaluationHit{ChunkID: hit.Metadata["chunk_id"], KnowledgeScope: hit.KnowledgeScope}
	}
	return result, nil
}

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local project config")
	alias := flag.String("alias", "", "stable Qdrant alias")
	goldenPath := flag.String("golden", "internal/documentingest/testdata/golden_queries.json", "golden case JSON")
	k := flag.Int("k", 3, "fixed retrieval cutoff")
	flag.Parse()
	if *k != 3 {
		log.Fatal("--k must be exactly 3 for Recall@3/MRR@3")
	}
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
	if *alias != required("DOCUMENT_INGEST_ALIAS") {
		log.Fatal("--alias must equal DOCUMENT_INGEST_ALIAS")
	}
	content, err := os.ReadFile(*goldenPath)
	if err != nil {
		log.Fatal(err)
	}
	var cases []documentingest.GoldenCase
	if err := json.Unmarshal(content, &cases); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	report, err := documentingest.Evaluate(ctx, cases, required("OLLAMA_EMBED_MODEL"), memoryitem.NewHTTPOllamaEmbedder(required("OLLAMA_BASE_URL")), scopedDocumentSearcher{contextretrieval.NewDocumentQdrant(required("QDRANT_BASE_URL"), *alias)})
	if err != nil {
		log.Fatal(err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
	if !report.Passed {
		os.Exit(1)
	}
}
