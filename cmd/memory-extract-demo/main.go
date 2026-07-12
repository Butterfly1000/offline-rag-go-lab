package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"

	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/recentchat"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local Ollama config file")
	model := flag.String("model", "qwen:7b", "Ollama completion model")
	maxOutputTokens := flag.Int("max-output-tokens", 512, "maximum structured response tokens")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	ollamaURL := strings.TrimSpace(values["OLLAMA_BASE_URL"])
	if ollamaURL == "" {
		ollamaURL = "http://127.0.0.1:11434"
	}

	// These messages intentionally include an assistant guess. The extractor
	// may use it as context, but validation will reject it as memory evidence.
	messages := []memoryitem.SourceMessage{
		{ID: 101, SessionID: "memory-extract-demo", UserID: "memory-extract-user", Role: "user", Content: "我叫小黄，这个项目使用 Go。"},
		{ID: 102, SessionID: "memory-extract-demo", UserID: "memory-extract-user", Role: "assistant", Content: "你可能也喜欢 Rust。"},
		{ID: 103, SessionID: "memory-extract-demo", UserID: "memory-extract-user", Role: "user", Content: "教学要贴近真实操作，不要只做模拟。"},
		{ID: 104, SessionID: "memory-extract-demo", UserID: "memory-extract-user", Role: "user", Content: "当前项目只允许 commit，不要自动 push。"},
	}

	extractor := memoryitem.NewExtractor(recentchat.NewHTTPOllamaClient(ollamaURL))
	result, err := extractor.Extract(memoryitem.ExtractRequest{
		Model:           *model,
		UserID:          "memory-extract-user",
		SessionID:       "memory-extract-demo",
		Messages:        messages,
		MaxOutputTokens: *maxOutputTokens,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Raw JSON:\n%s\n", prettyJSON(result.RawJSON))
	fmt.Printf("Validated candidates: %d\n", len(result.Candidates))
	for _, candidate := range result.Candidates {
		fmt.Printf(
			"- %s %s/%s=%q confidence=%.2f sources=%v\n",
			candidate.Operation,
			candidate.Kind,
			candidate.Key,
			candidate.Value,
			candidate.Confidence,
			candidate.SourceMessageIDs,
		)
	}
}

func prettyJSON(raw []byte) string {
	var output bytes.Buffer
	if err := json.Indent(&output, raw, "", "  "); err != nil {
		return string(raw)
	}
	return output.String()
}
