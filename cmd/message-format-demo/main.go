package main

import (
	"flag"
	"fmt"
	"log"

	"offline-rag-go-lab/internal/chatprompt"
)

func main() {
	role := flag.String("role", "user", "message role: system, user, assistant, or tool")
	content := flag.String("content", "你好，解释 token。", "message content")
	flag.Parse()

	formatted, err := (chatprompt.QwenFormatter{}).FormatMessage(chatprompt.Message{
		Role:    *role,
		Content: *content,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Role: %s\n", *role)
	fmt.Printf("Content: %s\n", *content)
	fmt.Printf("Formatted message:\n%s", formatted)
}
