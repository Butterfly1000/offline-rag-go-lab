package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"offline-rag-go-lab/internal/memoryitem"
)

func main() {
	messages := []memoryitem.SourceMessage{
		{
			ID:        101,
			SessionID: "memory-validation",
			UserID:    "u-001",
			Role:      "user",
			Content:   "这个项目使用 Go。",
		},
		{
			ID:        102,
			SessionID: "memory-validation",
			UserID:    "u-001",
			Role:      "assistant",
			Content:   "你可能也喜欢 Rust。",
		},
	}

	// The model may produce inconsistent spaces and key casing. Validation
	// converts the key to a stable identity before a database can use it.
	valid, err := memoryitem.ValidateAndNormalizeCandidate("u-001", "memory-validation", memoryitem.Candidate{
		Operation:        memoryitem.OperationUpsert,
		Kind:             memoryitem.KindProjectFact,
		Key:              " Implementation Language ",
		Value:            " Go ",
		Confidence:       0.95,
		SourceMessageIDs: []int64{101},
	}, messages)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Valid candidate: %s/%s=%s\n", valid.Kind, valid.Key, valid.Value)
	fmt.Printf("Sources: %s\n", joinIDs(valid.SourceMessageIDs))

	// Assistant text is useful conversational context, but it cannot be the
	// sole evidence for a durable fact about the user.
	_, err = memoryitem.ValidateAndNormalizeCandidate("u-001", "memory-validation", memoryitem.Candidate{
		Operation:        memoryitem.OperationUpsert,
		Kind:             memoryitem.KindPreference,
		Key:              "secondary_language",
		Value:            "Rust",
		Confidence:       0.8,
		SourceMessageIDs: []int64{102},
	}, messages)
	if err == nil {
		log.Fatal("assistant-only candidate should have been rejected")
	}
	fmt.Printf("Rejected assistant-only source: %v\n", err)
}

func joinIDs(ids []int64) string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, strconv.FormatInt(id, 10))
	}
	return strings.Join(values, ",")
}
