package main

import (
	"fmt"
	"log"

	"offline-rag-go-lab/internal/memoryitem"
)

func main() {
	steps := []memoryitem.Candidate{
		candidate(memoryitem.OperationUpsert, "Go", 101),
		candidate(memoryitem.OperationUpsert, " go ", 102),
		candidate(memoryitem.OperationUpsert, "Rust", 103),
		candidate(memoryitem.OperationForget, "", 104),
		candidate(memoryitem.OperationUpsert, "Go", 105),
	}

	var current *memoryitem.Item
	for _, nextCandidate := range steps {
		decision, err := memoryitem.Resolve(current, nextCandidate)
		if err != nil {
			log.Fatal(err)
		}

		// MySQL assigns identity fields after INSERT. The demo fills them only so
		// the next in-memory step has the same shape as a persisted item.
		next := decision.Next
		if decision.Action == memoryitem.ActionInsert {
			next.ID = 7
			next.UserID = "memory-resolve-user"
		}
		fmt.Printf(
			"%-6s version=%d status=%s value=%q reason=%s\n",
			decision.Action,
			next.Version,
			next.Status,
			next.Value,
			decision.Reason,
		)
		current = &next
	}
}

func candidate(operation memoryitem.Operation, value string, sourceID int64) memoryitem.Candidate {
	return memoryitem.Candidate{
		Operation:        operation,
		Kind:             memoryitem.KindProjectFact,
		Key:              "implementation_language",
		Value:            value,
		Confidence:       0.95,
		SourceMessageIDs: []int64{sourceID},
	}
}
