package contextretrieval

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

type runeCounter struct {
	err         error
	countOffset int
	calls       int
}

func (c *runeCounter) CountText(text string) (int, []string, []int, error) {
	c.calls++
	if c.err != nil {
		return 0, nil, nil, c.err
	}
	return utf8.RuneCountInString(text) + c.countOffset*c.calls, nil, nil, nil
}

func TestSelectWithinTokenBudgetSkipsOversizedAndKeepsLaterCandidate(t *testing.T) {
	large := validMemoryHit()
	large.ID = "memory:large"
	large.Content = strings.Repeat("大", 300)
	small := validDocumentHit()
	small.ID = "document:small"
	small.Content = "Go"
	smallRendered, err := RenderContext([]Hit{small})
	if err != nil {
		t.Fatal(err)
	}
	budget := utf8.RuneCountInString(smallRendered)

	selection, err := SelectWithinTokenBudget([]Hit{large, small}, budget, &runeCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Hits) != 1 || selection.Hits[0].ID != small.ID || selection.UsedTokens > budget {
		t.Fatalf("selection = %#v budget=%d", selection, budget)
	}
	if len(selection.DroppedIDs) != 1 || selection.DroppedIDs[0] != large.ID {
		t.Fatalf("dropped IDs = %v", selection.DroppedIDs)
	}
}

func TestSelectWithinTokenBudgetPreservesDroppedOrderAndCopiesHits(t *testing.T) {
	first := validMemoryHit()
	first.ID = "memory:first"
	first.Content = strings.Repeat("x", 200)
	first.Metadata = map[string]string{"key": "first"}
	second := validDocumentHit()
	second.ID = "document:second"
	second.Content = strings.Repeat("y", 200)
	selection, err := SelectWithinTokenBudget([]Hit{first, second}, 10, &runeCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(selection.DroppedIDs, ",") != "memory:first,document:second" || len(selection.Hits) != 0 || selection.Rendered != "" || selection.UsedTokens != 0 {
		t.Fatalf("selection = %#v", selection)
	}
}

func TestSelectWithinTokenBudgetRejectsInvalidInputsAndPropagatesCounterError(t *testing.T) {
	if _, err := SelectWithinTokenBudget(nil, 0, &runeCounter{}); err == nil {
		t.Fatal("zero budget error = nil")
	}
	if _, err := SelectWithinTokenBudget(nil, 1, nil); err == nil {
		t.Fatal("nil counter error = nil")
	}
	if _, err := SelectWithinTokenBudget([]Hit{validMemoryHit()}, 100, &runeCounter{err: errors.New("tokenizer failed")}); err == nil || !strings.Contains(err.Error(), "tokenizer failed") {
		t.Fatalf("counter error = %v", err)
	}
	empty, err := SelectWithinTokenBudget(nil, 1, &runeCounter{})
	if err != nil || len(empty.Hits) != 0 || empty.Rendered != "" || empty.UsedTokens != 0 {
		t.Fatalf("empty selection = %#v error=%v", empty, err)
	}
}

func TestSelectWithinTokenBudgetRejectsNondeterministicCounter(t *testing.T) {
	_, err := SelectWithinTokenBudget([]Hit{validMemoryHit()}, 1000, &runeCounter{countOffset: 1})
	if err == nil || !strings.Contains(err.Error(), "not deterministic") {
		t.Fatalf("nondeterministic counter error = %v", err)
	}
}
