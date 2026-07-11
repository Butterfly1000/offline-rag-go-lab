package main

import (
	"flag"
	"fmt"
	"log"

	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/sessionsummary"
)

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:11434", "Ollama base URL")
	model := flag.String("model", "qwen:7b", "Ollama model")
	previous := flag.String("previous", "用户叫小黄，代码示例使用 Go。", "previous session summary")
	maxTokens := flag.Int("max-tokens", 256, "maximum generated summary tokens")
	flag.Parse()

	messages := []sessionsummary.SourceMessage{
		{ID: 21, Role: "user", Content: "不要只讲模拟方案，要真实落地。"},
		{ID: 22, Role: "assistant", Content: "已完成 token 自动预算并接入 /chat。"},
		{ID: 23, Role: "user", Content: "下一步继续实现 session summary。"},
	}
	generated, err := sessionsummary.NewGenerator(
		recentchat.NewHTTPOllamaClient(*baseURL),
	).Update(*model, *previous, messages, *maxTokens)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Previous summary: %s\n", *previous)
	fmt.Printf("New message IDs: 21,22,23\n")
	fmt.Printf("Updated summary:\n%s\n", generated)
}
