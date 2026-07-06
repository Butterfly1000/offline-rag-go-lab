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

	text := "我叫小黄，这个项目是 Go 写的。后面的例子，请继续用 Go 来说明。"
	messages := []tokenizerdemo.Message{
		{Role: "user", Content: "我叫小黄。"},
		{Role: "assistant", Content: "这个项目是 Go 写的。"},
		{Role: "user", Content: "后面的例子，请继续用 Go 来说明。"},
	}

	counter := tokenizerdemo.NewCounter(*tokenizerPath)

	textCount, textTokens, textIDs, err := counter.CountText(text)
	if err != nil {
		log.Fatal(err)
	}

	perMessage, total, transcriptCount, err := counter.CountMessages(messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Tokenizer path: %s\n\n", *tokenizerPath)

	fmt.Println("== Text Demo ==")
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Token count: %d\n", textCount)
	fmt.Printf("First tokens: %v\n", clipStrings(textTokens, 20))
	fmt.Printf("First token ids: %v\n\n", clipInts(textIDs, 20))
	fmt.Println("Note: byte-level/BPE tokenizer 的 token 片段可能看起来像乱码，这是正常现象。")
	fmt.Println()

	fmt.Println("== Messages Demo ==")
	for i, msg := range messages {
		fmt.Printf("%d. %s: %s\n", i+1, msg.Role, msg.Content)
		fmt.Printf("   content-only token count: %d\n", perMessage[i])
	}
	fmt.Printf("\nMessages content-only total: %d\n", total)
	fmt.Printf("Combined transcript token count: %d\n", transcriptCount)
	fmt.Println("\nNote: current demo counts plain text and a simple combined transcript.")
	fmt.Println("It does not yet apply the model's full chat template.")
}

func clipStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func clipInts(values []int, limit int) []int {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
