package main

import (
	"fmt"
	"log"

	"offline-rag-go-lab/internal/contextretrieval"
)

func main() {
	memory, err := contextretrieval.ValidateHit(contextretrieval.Hit{
		Source:  contextretrieval.SourceMemory,
		ID:      "memory:7",
		Content: "project_fact/implementation_language: Go",
		Score:   0.91,
		UserID:  "u-001",
		Kind:    "project_fact",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Valid memory: source=%s user_id=%s\n", memory.Source, memory.UserID)

	document, err := contextretrieval.ValidateHit(contextretrieval.Hit{
		Source:         contextretrieval.SourceDocument,
		ID:             "document:chunk-001",
		Content:        "这个项目使用 Go 实现。",
		Score:          0.87,
		KnowledgeScope: "offline-rag-course",
		Title:          "项目实现说明",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Valid document: source=%s knowledge_scope=%s\n", document.Source, document.KnowledgeScope)

	// A memory result cannot claim a document scope. Treating this as a hard
	// error prevents a malformed retrieval payload from entering the prompt.
	mixed := memory
	mixed.KnowledgeScope = "offline-rag-course"
	if _, err := contextretrieval.ValidateHit(mixed); err != nil {
		fmt.Printf("Rejected mixed ownership: %v\n", err)
		return
	}
	log.Fatal("mixed ownership was not rejected")
}
