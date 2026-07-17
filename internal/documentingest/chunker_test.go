package documentingest

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

type runeTokenCounter struct{}

func (runeTokenCounter) Count(text string) (int, error) {
	return utf8.RuneCountInString(text), nil
}

type failingTokenCounter struct{}

func (failingTokenCounter) Count(string) (int, error) {
	return 0, fmt.Errorf("counter failed")
}

type nonMonotonicTokenCounter struct{}

func (nonMonotonicTokenCounter) Count(text string) (int, error) {
	if text == "a" {
		return 2, nil
	}
	if text == "ab" {
		return 1, nil
	}
	return utf8.RuneCountInString(text), nil
}

type mappedTokenCounter map[string]int

func (c mappedTokenCounter) Count(text string) (int, error) {
	if count, ok := c[text]; ok {
		return count, nil
	}
	return utf8.RuneCountInString(text), nil
}

type zeroTokenCounter struct{}

func (zeroTokenCounter) Count(string) (int, error) { return 0, nil }

func TestChunkDocumentAssignsDeterministicIdentityAndOrder(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\nSame.\n\nSame.")
	policy := ChunkPolicy{MaxTokens: 8, OverlapLines: 1}
	first, err := ChunkDocument(document, policy, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ChunkDocument(document, policy, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("chunk counts = %d and %d, want 2", len(first), len(second))
	}
	for index := range first {
		if first[index].Ordinal != index || first[index].ChunkID != second[index].ChunkID {
			t.Fatalf("chunk %d is not deterministic: %#v / %#v", index, first[index], second[index])
		}
		if first[index].TokenCount <= 0 || first[index].TokenCount > policy.MaxTokens {
			t.Fatalf("chunk %d token count = %d", index, first[index].TokenCount)
		}
	}
	if first[0].ChunkID == first[1].ChunkID {
		t.Fatal("identical repeated blocks must receive distinct chunk IDs")
	}
}

func TestChunkDocumentSplitsOversizedParagraphByExactCounter(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\n第一句很长。第二句也很长。第三句结束。")
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 8}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("chunks = %d, want multiple exact-token chunks", len(chunks))
	}
	for _, chunk := range chunks {
		actual, _ := runeTokenCounter{}.Count(chunk.Text)
		if chunk.TokenCount != actual || actual > 8 {
			t.Fatalf("chunk count = %d actual = %d text = %q", chunk.TokenCount, actual, chunk.Text)
		}
	}
}

func TestSplitTextExactDoesNotAssumeBPECountsAreMonotonic(t *testing.T) {
	pieces, err := splitTextExact("ab", 1, nonMonotonicTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 1 || pieces[0] != "ab" {
		t.Fatalf("pieces = %#v, want [ab]", pieces)
	}
}

func TestSplitLinesDoesNotStopAtFirstNonMonotonicCount(t *testing.T) {
	counter := mappedTokenCounter{"a": 1, "a\nb": 2, "a\nb\nc": 1}
	pieces, err := splitLines([]string{"a", "b", "c"}, "", "", ChunkPolicy{MaxTokens: 1}, counter)
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 1 || pieces[0] != "a\nb\nc" {
		t.Fatalf("pieces = %#v, want one complete candidate", pieces)
	}
}

func TestSplitLinesCountsWrappedCandidateInsteadOfAddingOverhead(t *testing.T) {
	counter := mappedTokenCounter{"P\nS": 2, "P\nab\nS": 2, "P\na\nS": 1, "P\nb\nS": 1}
	pieces, err := splitLines([]string{"ab"}, "P", "S", ChunkPolicy{MaxTokens: 1}, counter)
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 2 || pieces[0] != "P\na\nS" || pieces[1] != "P\nb\nS" {
		t.Fatalf("pieces = %#v, want wrapped exact fragments", pieces)
	}
}

func TestSplitLinesDoesNotEmitOverlapOnlyChunk(t *testing.T) {
	pieces, err := splitLines(
		[]string{"a", "bbbb", "cccc"}, "", "",
		ChunkPolicy{MaxTokens: 6, OverlapLines: 1}, runeTokenCounter{},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a\nbbbb", "cccc"}
	if len(pieces) != len(want) {
		t.Fatalf("pieces = %#v, want %#v", pieces, want)
	}
	for index := range want {
		if pieces[index] != want[index] {
			t.Fatalf("piece %d = %q, want %q", index, pieces[index], want[index])
		}
	}
}

func TestCountTokensRejectsZeroForNonEmptyText(t *testing.T) {
	_, err := countTokens(zeroTokenCounter{}, "non-empty")
	if err == nil || !strings.Contains(err.Error(), "zero") {
		t.Fatalf("countTokens() error = %v", err)
	}
}

func TestChunkDocumentRejectsInvalidPolicyCounterAndFormat(t *testing.T) {
	valid := normalizedTestDocument(t, FormatMarkdown, "# Course\n\nText")
	tests := []struct {
		name    string
		doc     Document
		policy  ChunkPolicy
		counter TokenCounter
		want    string
	}{
		{name: "max tokens", doc: valid, policy: ChunkPolicy{}, counter: runeTokenCounter{}, want: "max_tokens"},
		{name: "negative overlap", doc: valid, policy: ChunkPolicy{MaxTokens: 10, OverlapLines: -1}, counter: runeTokenCounter{}, want: "overlap_lines"},
		{name: "overlap without room", doc: valid, policy: ChunkPolicy{MaxTokens: 1, OverlapLines: 1}, counter: runeTokenCounter{}, want: "overlap"},
		{name: "nil counter", doc: valid, policy: ChunkPolicy{MaxTokens: 10}, counter: nil, want: "counter"},
		{name: "unsupported format", doc: Document{KnowledgeScope: "scope", DocumentID: "doc", SourceRef: "a.txt", Format: "text", Content: []byte("text")}, policy: ChunkPolicy{MaxTokens: 10}, counter: runeTokenCounter{}, want: "unsupported"},
		{name: "counter error", doc: valid, policy: ChunkPolicy{MaxTokens: 10}, counter: failingTokenCounter{}, want: "counter failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ChunkDocument(tt.doc, tt.policy, tt.counter)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ChunkDocument() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func normalizedTestDocument(t *testing.T, format DocumentFormat, content string) Document {
	t.Helper()
	document, err := NormalizeDocument(Document{
		KnowledgeScope: "document-ingestion-course",
		DocumentID:     "course-source",
		SourceRef:      "internal/documentingest/testdata/course.txt",
		Format:         format,
		Content:        []byte(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	return document
}
