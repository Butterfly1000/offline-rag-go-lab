package documentingest

import (
	"strings"
	"testing"
)

func TestMarkdownChunkingUsesHeadingStackAndParagraphs(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, `# Course

Introduction paragraph.

## Identity

Identity paragraph.

### Hash

Hash paragraph.`)
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 200}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks = %#v, want 3", chunks)
	}
	wantPaths := []string{"Course", "Course / Identity", "Course / Identity / Hash"}
	for index, want := range wantPaths {
		if chunks[index].HeadingPath != want || chunks[index].StructureKind != "paragraph" {
			t.Fatalf("chunk %d = %#v, want paragraph under %q", index, chunks[index], want)
		}
	}
}

func TestMarkdownChunkingPreservesFencedCode(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\nBefore.\n\n```go\nfunc main() {}\n```\n\nAfter.")
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 200}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks = %#v, want paragraph/code/paragraph", chunks)
	}
	code := chunks[1]
	if code.StructureKind != "code" || code.HeadingPath != "Course" {
		t.Fatalf("code chunk = %#v", code)
	}
	if code.Text != "```go\nfunc main() {}\n```" {
		t.Fatalf("fenced code = %q", code.Text)
	}
}

func TestMarkdownChunkingSplitsOversizedFenceWithLanguageMarkerAndOverlap(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\n```go\nline one\nline two\nline three\nline four\n```")
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 30, OverlapLines: 1}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks = %#v, want split fence", chunks)
	}
	for _, chunk := range chunks {
		if chunk.StructureKind != "code" || !strings.HasPrefix(chunk.Text, "```go\n") || !strings.HasSuffix(chunk.Text, "\n```") {
			t.Fatalf("split code lost fence: %q", chunk.Text)
		}
		if chunk.TokenCount > 30 {
			t.Fatalf("split code token count = %d", chunk.TokenCount)
		}
	}
	if !strings.Contains(chunks[0].Text, "line two") || !strings.Contains(chunks[1].Text, "line two") {
		t.Fatalf("expected one overlapped complete line: %#v", chunks)
	}
}

func TestMarkdownChunkingRejectsUnclosedFence(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\n```go\nfunc main() {}")
	_, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 100}, runeTokenCounter{})
	if err == nil || !strings.Contains(err.Error(), "unclosed") {
		t.Fatalf("ChunkDocument() error = %v", err)
	}
}

func TestMarkdownDoesNotTreatOrdinaryHashTextAsHeading(t *testing.T) {
	document := normalizedTestDocument(t, FormatMarkdown, "# Course\n\nC# is a language.\nThe value #1 remains text.")
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 200}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].HeadingPath != "Course" || !strings.Contains(chunks[0].Text, "#1") {
		t.Fatalf("chunks = %#v", chunks)
	}
}
