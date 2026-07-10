package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"offline-rag-go-lab/internal/recentchat"
)

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:11434", "Ollama base URL")
	model := flag.String("model", "qwen:7b", "Ollama model name")
	flag.Parse()

	client := recentchat.NewHTTPOllamaClient(*baseURL)
	summary, err := client.Show(*model)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model: %s\n", summary.Model)
	fmt.Printf("Family: %s\n", display(summary.Family))
	fmt.Printf("Architecture: %s\n", display(summary.Architecture))
	fmt.Printf("Parameter size: %s\n", display(summary.ParameterSize))
	fmt.Printf("Quantization: %s\n", display(summary.QuantizationLevel))
	fmt.Printf("Context length: %d\n", summary.ContextLength)
	fmt.Printf("Capabilities: %s\n", display(strings.Join(summary.Capabilities, ", ")))
	fmt.Printf("Parameters:\n%s\n", display(summary.Parameters))
	fmt.Printf("Template:\n%s\n", display(summary.Template))
}

func display(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(not reported)"
	}
	return value
}
