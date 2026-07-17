package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"offline-rag-go-lab/internal/documentingest"
	"offline-rag-go-lab/internal/fileconfig"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local KEY=VALUE config file")
	formatFlag := flag.String("format", "", "document format: markdown or go")
	sourcePath := flag.String("source", "", "repository-relative source path")
	maxTokens := flag.Int("max-tokens", 160, "hard token ceiling for every chunk")
	overlapLines := flag.Int("overlap-lines", 2, "complete lines repeated when an oversized structure is split")
	knowledgeScope := flag.String("scope", "document-ingestion-course", "knowledge scope used in stable chunk IDs")
	documentID := flag.String("document-id", "chunk-demo", "logical document ID used in stable chunk IDs")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	tokenizerPath, err := fileconfig.Required(values, "RECENT_CHAT_TOKENIZER_PATH")
	if err != nil {
		log.Fatal(err)
	}
	if strings.TrimSpace(*sourcePath) == "" {
		log.Fatal("--source is required")
	}
	content, err := os.ReadFile(*sourcePath)
	if err != nil {
		log.Fatalf("read source %s: %v", *sourcePath, err)
	}
	counter, err := documentingest.NewQwenTokenCounter(tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}

	chunks, err := documentingest.ChunkDocument(documentingest.Document{
		KnowledgeScope: *knowledgeScope,
		DocumentID:     *documentID,
		SourceRef:      *sourcePath,
		Format:         documentingest.DocumentFormat(*formatFlag),
		Content:        content,
	}, documentingest.ChunkPolicy{MaxTokens: *maxTokens, OverlapLines: *overlapLines}, counter)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Source: %s\n", *sourcePath)
	fmt.Printf("Format: %s\n", strings.ToLower(strings.TrimSpace(*formatFlag)))
	fmt.Printf("Tokenizer: %s\n", tokenizerPath)
	fmt.Printf("Policy: max_tokens=%d overlap_lines=%d\n", *maxTokens, *overlapLines)
	fmt.Printf("Chunks: %d\n", len(chunks))
	for _, chunk := range chunks {
		fmt.Printf("\n[%d] kind=%s tokens=%d\n", chunk.Ordinal, chunk.StructureKind, chunk.TokenCount)
		fmt.Printf("path=%s\n", chunk.HeadingPath)
		fmt.Printf("chunk_id=%s\n", chunk.ChunkID)
		fmt.Printf("preview=%s\n", preview(chunk.Text, 90))
	}
}

func preview(text string, limit int) string {
	text = strings.ReplaceAll(text, "\n", `\n`)
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "..."
}
