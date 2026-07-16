package contextretrieval

import (
	"regexp"
	"strings"
	"testing"
)

func TestNormalizeDocumentChunkRequiresStableIdentityAndContent(t *testing.T) {
	valid := DocumentChunk{
		KnowledgeScope: " offline-rag-course ", DocumentID: " project-overview ",
		ChunkID: " chunk-001 ", Title: " 项目说明 ", SourceRef: " docs/project.md ",
		Text: " 这个项目使用 Go。 ",
	}
	got, err := normalizeDocumentChunk(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.KnowledgeScope != "offline-rag-course" || got.DocumentID != "project-overview" || got.Text != "这个项目使用 Go。" {
		t.Fatalf("normalized chunk = %#v", got)
	}

	tests := []struct {
		name string
		edit func(*DocumentChunk)
		want string
	}{
		{name: "scope", edit: func(c *DocumentChunk) { c.KnowledgeScope = "" }, want: "knowledge_scope is required"},
		{name: "document", edit: func(c *DocumentChunk) { c.DocumentID = "" }, want: "document_id is required"},
		{name: "chunk", edit: func(c *DocumentChunk) { c.ChunkID = "" }, want: "chunk_id is required"},
		{name: "text", edit: func(c *DocumentChunk) { c.Text = "" }, want: "text is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.edit(&input)
			_, err := normalizeDocumentChunk(input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("normalizeDocumentChunk() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestDeterministicDocumentPointIDIncludesScopeAndChunk(t *testing.T) {
	first, err := DeterministicDocumentPointID("offline-rag-course", "chunk-001")
	if err != nil {
		t.Fatal(err)
	}
	second, err := DeterministicDocumentPointID("offline-rag-course", "chunk-001")
	if err != nil {
		t.Fatal(err)
	}
	otherScope, err := DeterministicDocumentPointID("another-course", "chunk-001")
	if err != nil {
		t.Fatal(err)
	}
	otherChunk, err := DeterministicDocumentPointID("offline-rag-course", "chunk-002")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == otherScope || first == otherChunk {
		t.Fatalf("point IDs first=%q second=%q scope=%q chunk=%q", first, second, otherScope, otherChunk)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(first) {
		t.Fatalf("point ID is not a v4-layout UUID: %q", first)
	}
	if _, err := DeterministicDocumentPointID("", "chunk-001"); err == nil {
		t.Fatal("empty scope error = nil")
	}
}
