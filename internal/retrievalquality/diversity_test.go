package retrievalquality

import (
	"reflect"
	"strings"
	"testing"
)

func TestApplyDiversityCapsDocumentsAndHeadingsWithoutReordering(t *testing.T) {
	input := []Candidate{
		{KnowledgeScope: "s", DocumentID: "d1", HeadingPath: "h1", ChunkID: "a"},
		{KnowledgeScope: "s", DocumentID: "d1", HeadingPath: "h1", ChunkID: "b"},
		{KnowledgeScope: "s", DocumentID: "d1", HeadingPath: "h1", ChunkID: "c"},
		{KnowledgeScope: "s", DocumentID: "d1", HeadingPath: "h2", ChunkID: "d"},
		{KnowledgeScope: "s", DocumentID: "d2", HeadingPath: "h3", ChunkID: "e"},
	}
	kept, skipped, err := ApplyDiversity(input, DiversityLimits{PerDocument: 3, PerHeading: 2})
	if err != nil {
		t.Fatal(err)
	}
	var keptIDs, skippedIDs []string
	for _, item := range kept {
		keptIDs = append(keptIDs, item.ChunkID)
	}
	for _, item := range skipped {
		skippedIDs = append(skippedIDs, item.ChunkID+":"+item.Reason)
	}
	if want := []string{"a", "b", "d", "e"}; !reflect.DeepEqual(keptIDs, want) {
		t.Fatalf("kept=%v, want %v", keptIDs, want)
	}
	if len(skippedIDs) != 1 || !strings.Contains(skippedIDs[0], "heading_limit") {
		t.Fatalf("skipped=%v", skippedIDs)
	}
	if len(input) != 5 || input[2].ChunkID != "c" {
		t.Fatalf("input was mutated: %v", input)
	}
}

func TestApplyDiversityRejectsMissingOwnershipOrMixedScope(t *testing.T) {
	for _, input := range [][]Candidate{
		{{KnowledgeScope: "s", DocumentID: "", HeadingPath: "h", ChunkID: "a"}},
		{
			{KnowledgeScope: "s1", DocumentID: "d1", HeadingPath: "h1", ChunkID: "a"},
			{KnowledgeScope: "s2", DocumentID: "d2", HeadingPath: "h2", ChunkID: "b"},
		},
	} {
		if _, _, err := ApplyDiversity(input, DiversityLimits{PerDocument: 3, PerHeading: 2}); err == nil {
			t.Fatalf("input=%v should fail", input)
		}
	}
}
