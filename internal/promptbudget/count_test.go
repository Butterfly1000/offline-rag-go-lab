package promptbudget

import (
	"errors"
	"strings"
	"testing"
)

type fakeTokenCounter struct {
	counts map[string]int
	errFor string
}

func (f fakeTokenCounter) CountText(text string) (int, []string, []int, error) {
	if text == f.errFor {
		return 0, nil, nil, errors.New("count failed")
	}
	return f.counts[text], nil, nil, nil
}

func TestCompareTokensReturnsContentRenderedAndOverheadCounts(t *testing.T) {
	counter := fakeTokenCounter{counts: map[string]int{
		"system":   5,
		"prompt":   3,
		"rendered": 12,
	}}

	comparison, err := CompareTokens(counter, "system", "prompt", "rendered")
	if err != nil {
		t.Fatalf("CompareTokens() error = %v", err)
	}
	want := TokenComparison{
		SystemTokens:     5,
		PromptTokens:     3,
		ContentTokens:    8,
		RenderedTokens:   12,
		TemplateOverhead: 4,
	}
	if comparison != want {
		t.Fatalf("CompareTokens() = %+v, want %+v", comparison, want)
	}
}

func TestCompareTokensReturnsCountingError(t *testing.T) {
	counter := fakeTokenCounter{
		counts: map[string]int{"system": 5, "prompt": 3},
		errFor: "rendered",
	}

	_, err := CompareTokens(counter, "system", "prompt", "rendered")
	if err == nil {
		t.Fatal("CompareTokens() error = nil, want rendered count error")
	}
	if !strings.Contains(err.Error(), "rendered prompt") {
		t.Fatalf("CompareTokens() error = %q, want rendered prompt context", err)
	}
}
