package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/promptbudget"
	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:11434", "Ollama base URL")
	model := flag.String("model", "qwen:7b", "Ollama model name")
	system := flag.String("system", "你是 Go 助手。", "system message")
	prompt := flag.String("prompt", "解释 recent window。", "current user message")
	outputReserve := flag.Int("output-reserve", 2048, "tokens reserved for the model response")
	tokenizerPath := flag.String("tokenizer", filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json"), "path to tokenizer.json")
	flag.Parse()

	ollama := recentchat.NewHTTPOllamaClient(*baseURL)
	rawCounter, err := tokenizerdemo.LoadCounter(*tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}
	conversationCounter := chatprompt.NewTokenCounter(rawCounter, chatprompt.QwenFormatter{})
	planner := promptbudget.NewAutomaticPlanner(ollama, conversationCounter)

	fixed := make([]chatprompt.Message, 0, 2)
	if *system != "" {
		fixed = append(fixed, chatprompt.Message{Role: "system", Content: *system})
	}
	fixed = append(fixed, chatprompt.Message{Role: "user", Content: *prompt})

	plan, err := planner.Plan(*model, fixed, *outputReserve)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model: %s\n", *model)
	fmt.Printf("Rendered fixed prompt:\n%s", plan.RenderedFixedPrompt)
	fmt.Printf("Context limit: %d\n", plan.ContextLimit)
	fmt.Printf("Fixed input tokens: %d\n", plan.FixedInputTokens)
	fmt.Printf("Output reserve tokens: %d\n", plan.OutputReserve)
	fmt.Printf("Available recent history tokens: %d\n", plan.AvailableHistoryTokens)
}
