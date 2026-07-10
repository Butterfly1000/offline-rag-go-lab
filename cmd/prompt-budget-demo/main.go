package main

import (
	"flag"
	"fmt"
	"log"

	"offline-rag-go-lab/internal/promptbudget"
	"offline-rag-go-lab/internal/recentchat"
)

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:11434", "Ollama base URL")
	model := flag.String("model", "qwen:7b", "Ollama model name")
	system := flag.String("system", "你是一个 Go 项目教学助手。", "system prompt")
	prompt := flag.String("prompt", "解释 token 是如何计算的。", "user prompt")
	flag.Parse()

	client := recentchat.NewHTTPOllamaClient(*baseURL)
	modelSummary, err := client.Show(*model)
	if err != nil {
		log.Fatal(err)
	}

	rendered, err := promptbudget.Render(modelSummary.Template, *system, *prompt)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model: %s\n", modelSummary.Model)
	fmt.Printf("Context length: %d\n", modelSummary.ContextLength)
	fmt.Printf("System: %s\n", *system)
	fmt.Printf("Prompt: %s\n", *prompt)
	fmt.Printf("Rendered prompt:\n%s\n", rendered)
}
