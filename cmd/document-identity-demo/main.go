package main

import (
	"fmt"
	"log"

	"offline-rag-go-lab/internal/documentingest"
)

func main() {
	base := documentingest.ChunkIdentityInput{
		KnowledgeScope: "document-ingestion-course",
		DocumentID:     "course-markdown",
		StructureKind:  "paragraph",
		HeadingPath:    "Course / Identity",
		Content:        "稳定 chunk ID 不包含文档版本和全局行号。",
	}

	unchanged := base
	changed := base
	changed.Content = "稳定 chunk ID 只在内容保持不变时复用。"
	moved := base
	moved.HeadingPath = "Course / Versioning"
	duplicate := base
	duplicate.DuplicateOrdinal = 1

	baseID := mustChunkID(base)
	unchangedID := mustChunkID(unchanged)
	changedID := mustChunkID(changed)
	movedID := mustChunkID(moved)
	duplicateID := mustChunkID(duplicate)

	policyHash, err := documentingest.ChunkPolicyHash(documentingest.ChunkPolicyIdentity{
		Format: documentingest.FormatMarkdown, ParserVersion: "markdown-v1", MaxTokens: 160, OverlapLines: 2,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("== Stable Chunk Identity ==")
	fmt.Printf("Base:      %s\n", baseID)
	fmt.Printf("Unchanged: %s (same=%t)\n", unchangedID, unchangedID == baseID)
	fmt.Printf("Changed:   %s (same=%t)\n", changedID, changedID == baseID)
	fmt.Printf("Moved:     %s (same=%t)\n", movedID, movedID == baseID)
	fmt.Printf("Duplicate: %s (same=%t)\n", duplicateID, duplicateID == baseID)
	fmt.Println()
	fmt.Println("== Version Build Identity ==")
	fmt.Printf("Content hash: %s\n", documentingest.ContentHash([]byte("# Course\r\n\r\nIdentity.  \r\n")))
	fmt.Printf("Policy hash:  %s\n", policyHash)
	fmt.Println()
	fmt.Println("== Allowed Version Transitions ==")
	for _, transition := range [][2]documentingest.VersionStatus{
		{documentingest.StatusPending, documentingest.StatusBuilding},
		{documentingest.StatusBuilding, documentingest.StatusReady},
		{documentingest.StatusReady, documentingest.StatusActive},
		{documentingest.StatusBuilding, documentingest.StatusFailed},
		{documentingest.StatusFailed, documentingest.StatusBuilding},
	} {
		if err := documentingest.ValidateTransition(transition[0], transition[1]); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s -> %s\n", transition[0], transition[1])
	}

	if err := documentingest.ValidateTransition(documentingest.StatusActive, documentingest.StatusBuilding); err != nil {
		fmt.Printf("Rejected: active -> building (%v)\n", err)
	}
}

func mustChunkID(input documentingest.ChunkIdentityInput) string {
	id, err := documentingest.StableChunkID(input)
	if err != nil {
		log.Fatal(err)
	}
	return id
}
