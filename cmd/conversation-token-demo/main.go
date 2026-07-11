package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	system := flag.String("system", "你是 Go 助手。", "system message")
	historyUser := flag.String("history-user", "我叫小黄。", "historical user message")
	historyAssistant := flag.String("history-assistant", "记住了。", "historical assistant message")
	prompt := flag.String("prompt", "我叫什么？", "current user message")
	tokenizerPath := flag.String("tokenizer", filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json"), "path to tokenizer.json")
	flag.Parse()

	messages := make([]chatprompt.Message, 0, 4)
	if *system != "" {
		messages = append(messages, chatprompt.Message{Role: "system", Content: *system})
	}
	if *historyUser != "" {
		messages = append(messages, chatprompt.Message{Role: "user", Content: *historyUser})
	}
	if *historyAssistant != "" {
		messages = append(messages, chatprompt.Message{Role: "assistant", Content: *historyAssistant})
	}
	messages = append(messages, chatprompt.Message{Role: "user", Content: *prompt})

	rawCounter, err := tokenizerdemo.LoadCounter(*tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}
	usage, err := chatprompt.NewTokenCounter(rawCounter, chatprompt.QwenFormatter{}).Count(messages, true)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Messages: %d\n", len(messages))
	fmt.Printf("Rendered conversation:\n%s", usage.Rendered)
	fmt.Printf("Total prompt tokens: %d\n", usage.TotalTokens)
}
