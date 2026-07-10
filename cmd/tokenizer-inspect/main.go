package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	defaultPath := filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json")
	tokenizerPath := flag.String("tokenizer", defaultPath, "path to tokenizer.json")
	flag.Parse()

	summary, err := tokenizerdemo.InspectFile(*tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Tokenizer path: %s\n", *tokenizerPath)
	fmt.Printf("Format version: %s\n", display(summary.Version))
	fmt.Printf("Model: %s\n", display(summary.ModelType))
	fmt.Printf("Normalizer: %s\n", display(summary.NormalizerType))
	fmt.Printf("Pre-tokenizer: %s\n", display(summary.PreTokenizerType))
	fmt.Printf("Post-processor: %s\n", display(summary.PostProcessorType))
	fmt.Printf("Decoder: %s\n", display(summary.DecoderType))
	fmt.Printf("Base vocab entries: %d\n", summary.VocabSize)
	fmt.Printf("Added tokens: %d\n", summary.AddedTokens)
	fmt.Println("Note: this summary describes the file; it does not prove which model produced it.")
}

func display(value string) string {
	if value == "" {
		return "(not configured)"
	}
	return value
}
