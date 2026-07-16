package contextretrieval

import (
	"reflect"
	"testing"
)

func TestMergeUsesSourceQuotasStableOrderAndCrossSourceDeduplication(t *testing.T) {
	memory := []Hit{
		{Source: SourceMemory, ID: "memory:2", Content: "Use Go", Score: 0.8, UserID: "u-001", Metadata: map[string]string{"key": "language"}},
		{Source: SourceMemory, ID: "memory:1", Content: "Keep tests", Score: 0.8, UserID: "u-001"},
		{Source: SourceMemory, ID: "memory:3", Content: "Third memory", Score: 0.7, UserID: "u-001"},
	}
	documents := []Hit{
		{Source: SourceDocument, ID: "document:2", Content: "  USE   go  ", Score: 0.99, KnowledgeScope: "course"},
		{Source: SourceDocument, ID: "document:3", Content: "Document B", Score: 0.6, KnowledgeScope: "course"},
		{Source: SourceDocument, ID: "document:1", Content: "Document A", Score: 0.6, KnowledgeScope: "course"},
	}
	memoryBefore := cloneHitsForTest(memory)
	documentsBefore := cloneHitsForTest(documents)

	got, err := Merge(memory, documents, MergeLimits{Memory: 2, Documents: 2})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"memory:1", "memory:2", "document:1", "document:3"}
	if idsOf(got) == nil || !reflect.DeepEqual(idsOf(got), wantIDs) {
		t.Fatalf("Merge() IDs = %v, want %v", idsOf(got), wantIDs)
	}
	if !reflect.DeepEqual(memory, memoryBefore) || !reflect.DeepEqual(documents, documentsBefore) {
		t.Fatalf("Merge() mutated inputs: memory=%#v documents=%#v", memory, documents)
	}
	got[1].Metadata["key"] = "changed"
	if memory[0].Metadata["key"] == "changed" {
		t.Fatal("Merge() retained caller metadata map")
	}
}

func TestMergeRejectsInvalidHitsAndLimits(t *testing.T) {
	if _, err := Merge(nil, nil, MergeLimits{Memory: -1}); err == nil {
		t.Fatal("negative limit error = nil")
	}
	invalid := validMemoryHit()
	invalid.Content = ""
	if _, err := Merge([]Hit{invalid}, nil, MergeLimits{Memory: 1}); err == nil {
		t.Fatal("invalid hit error = nil")
	}
}

func cloneHitsForTest(hits []Hit) []Hit {
	cloned := make([]Hit, len(hits))
	for index, hit := range hits {
		cloned[index] = hit
		if hit.Metadata != nil {
			cloned[index].Metadata = make(map[string]string, len(hit.Metadata))
			for key, value := range hit.Metadata {
				cloned[index].Metadata[key] = value
			}
		}
	}
	return cloned
}

func idsOf(hits []Hit) []string {
	ids := make([]string, len(hits))
	for index, hit := range hits {
		ids[index] = hit.ID
	}
	return ids
}
