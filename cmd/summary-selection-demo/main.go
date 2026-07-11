package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/sessionsummary"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	idsText := flag.String("ids", "19,20,21,22,23,24,25,26", "ascending message IDs")
	watermark := flag.Int64("watermark", 20, "last summarized message ID")
	recentStart := flag.Int64("recent-start", 25, "oldest recent-window message ID; 0 means no recent messages")
	tokenizerPath := flag.String("tokenizer", filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json"), "path to tokenizer.json")
	flag.Parse()

	ids, err := parseIDs(*idsText)
	if err != nil {
		log.Fatal(err)
	}
	messages := make([]sessionsummary.SourceMessage, 0, len(ids))
	for i, id := range ids {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages = append(messages, sessionsummary.SourceMessage{
			ID:      id,
			Role:    role,
			Content: fmt.Sprintf("message-%d", id),
		})
	}

	rawCounter, err := tokenizerdemo.LoadCounter(*tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}
	selection, err := sessionsummary.SelectPrefix(
		messages,
		*watermark,
		*recentStart,
		sessionsummary.NewFormattedMessageCounter(rawCounter, chatprompt.QwenFormatter{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Watermark: %d\n", *watermark)
	fmt.Printf("Recent start: %d\n", *recentStart)
	fmt.Printf("Unsummarized IDs: %s\n", formatIDs(selection.Unsummarized))
	fmt.Printf("Evicted IDs: %s\n", formatIDs(selection.Evicted))
	fmt.Printf("Unsummarized tokens: %d\n", selection.UnsummarizedTokens)
	fmt.Printf("Evicted tokens: %d\n", selection.EvictedTokens)
	fmt.Printf("Next watermark: %d\n", selection.NextWatermark)
}

func parseIDs(value string) ([]int64, error) {
	parts := strings.Split(value, ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse message ID %q: %w", part, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func formatIDs(messages []sessionsummary.SourceMessage) string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, strconv.FormatInt(message.ID, 10))
	}
	return strings.Join(ids, ",")
}
