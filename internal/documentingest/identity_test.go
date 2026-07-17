package documentingest

import (
	"strings"
	"testing"
)

func TestNormalizeDocument(t *testing.T) {
	document, err := NormalizeDocument(Document{
		KnowledgeScope: " document-ingestion-course ",
		DocumentID:     " course-markdown ",
		SourceRef:      " docs/teaching/course.md ",
		Format:         " MARKDOWN ",
		Content:        []byte("# Course\r\n\r\nContent.  \r\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.KnowledgeScope != "document-ingestion-course" || document.DocumentID != "course-markdown" {
		t.Fatalf("identity = %q/%q", document.KnowledgeScope, document.DocumentID)
	}
	if document.SourceRef != "docs/teaching/course.md" || document.Format != FormatMarkdown {
		t.Fatalf("source = %q format = %q", document.SourceRef, document.Format)
	}
	if string(document.Content) != "# Course\n\nContent." {
		t.Fatalf("normalized content = %q", document.Content)
	}

	// Normalization must copy the caller's bytes so later caller mutation cannot
	// silently change the version hash or chunks.
	input := []byte("original")
	document, err = NormalizeDocument(Document{
		KnowledgeScope: "scope", DocumentID: "document", SourceRef: "docs/a.md",
		Format: FormatMarkdown, Content: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	if string(document.Content) != "original" {
		t.Fatalf("normalized document aliases input: %q", document.Content)
	}
}

func TestNormalizeDocumentRejectsInvalidIdentityAndSource(t *testing.T) {
	valid := Document{
		KnowledgeScope: "scope", DocumentID: "document", SourceRef: "docs/a.md",
		Format: FormatMarkdown, Content: []byte("content"),
	}
	tests := []struct {
		name string
		edit func(*Document)
		want string
	}{
		{name: "scope empty", edit: func(d *Document) { d.KnowledgeScope = " " }, want: "knowledge_scope"},
		{name: "scope unsafe", edit: func(d *Document) { d.KnowledgeScope = "course one" }, want: "knowledge_scope"},
		{name: "document empty", edit: func(d *Document) { d.DocumentID = "" }, want: "document_id"},
		{name: "document unsafe", edit: func(d *Document) { d.DocumentID = "../document" }, want: "document_id"},
		{name: "source empty", edit: func(d *Document) { d.SourceRef = "" }, want: "source_ref"},
		{name: "source absolute", edit: func(d *Document) { d.SourceRef = "/tmp/a.md" }, want: "relative"},
		{name: "source traversal", edit: func(d *Document) { d.SourceRef = "docs/../../a.md" }, want: "escape"},
		{name: "source backslash", edit: func(d *Document) { d.SourceRef = `docs\a.md` }, want: "slash-separated"},
		{name: "source unclean", edit: func(d *Document) { d.SourceRef = "docs/./a.md" }, want: "clean"},
		{name: "source too long", edit: func(d *Document) { d.SourceRef = strings.Repeat("a", 1025) }, want: "1024"},
		{name: "source control", edit: func(d *Document) { d.SourceRef = "docs/a\x00.md" }, want: "control"},
		{name: "format empty", edit: func(d *Document) { d.Format = "" }, want: "format"},
		{name: "format unsupported", edit: func(d *Document) { d.Format = "pdf" }, want: "unsupported"},
		{name: "content empty", edit: func(d *Document) { d.Content = []byte(" \r\n") }, want: "content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document := valid
			tt.edit(&document)
			_, err := NormalizeDocument(document)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NormalizeDocument() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestContentHashNormalizesPortableText(t *testing.T) {
	unix := ContentHash([]byte("heading\ntext\n"))
	windows := ContentHash([]byte("heading  \r\ntext\r\n\r\n"))
	if unix != windows {
		t.Fatalf("portable hashes differ: %q != %q", unix, windows)
	}
	if len(unix) != 64 || strings.ToLower(unix) != unix {
		t.Fatalf("hash = %q, want lowercase SHA256 hex", unix)
	}
}

func TestStableChunkIDSurvivesDocumentVersionChange(t *testing.T) {
	input := validChunkIdentity("same text")
	first, err := StableChunkID(input)
	if err != nil {
		t.Fatal(err)
	}

	// A document version and global line number are deliberately absent from
	// ChunkIdentityInput, so unrelated insertions cannot rename this chunk.
	second, err := StableChunkID(input)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("stable IDs differ: %q != %q", first, second)
	}
	if len(first) != 64 || strings.ToLower(first) != first {
		t.Fatalf("chunk ID = %q, want lowercase SHA256 hex", first)
	}
}

func TestStableChunkIDChangesWithContentStructureAndDuplicate(t *testing.T) {
	base := validChunkIdentity("same text")
	want, err := StableChunkID(base)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*ChunkIdentityInput)
	}{
		{name: "content", edit: func(in *ChunkIdentityInput) { in.Content = "changed text" }},
		{name: "heading", edit: func(in *ChunkIdentityInput) { in.HeadingPath = "Course / Other" }},
		{name: "structure kind", edit: func(in *ChunkIdentityInput) { in.StructureKind = "code" }},
		{name: "document", edit: func(in *ChunkIdentityInput) { in.DocumentID = "other-document" }},
		{name: "scope", edit: func(in *ChunkIdentityInput) { in.KnowledgeScope = "other-scope" }},
		{name: "duplicate ordinal", edit: func(in *ChunkIdentityInput) { in.DuplicateOrdinal = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.edit(&input)
			got, err := StableChunkID(input)
			if err != nil {
				t.Fatal(err)
			}
			if got == want {
				t.Fatalf("changed %s retained ID %q", tt.name, got)
			}
		})
	}
}

func TestStableChunkIDNormalizesTextAndPath(t *testing.T) {
	first := validChunkIdentity("line  \r\n")
	first.HeadingPath = " Course / Identity "
	second := validChunkIdentity("line\n")
	second.HeadingPath = "Course / Identity"

	a, err := StableChunkID(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := StableChunkID(second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("normalized IDs differ: %q != %q", a, b)
	}
}

func TestStableChunkIDRejectsInvalidInput(t *testing.T) {
	valid := validChunkIdentity("text")
	tests := []struct {
		name string
		edit func(*ChunkIdentityInput)
		want string
	}{
		{name: "scope", edit: func(in *ChunkIdentityInput) { in.KnowledgeScope = "" }, want: "knowledge_scope"},
		{name: "document", edit: func(in *ChunkIdentityInput) { in.DocumentID = "" }, want: "document_id"},
		{name: "kind", edit: func(in *ChunkIdentityInput) { in.StructureKind = "" }, want: "structure_kind"},
		{name: "path", edit: func(in *ChunkIdentityInput) { in.HeadingPath = "" }, want: "heading_path"},
		{name: "content", edit: func(in *ChunkIdentityInput) { in.Content = "\r\n" }, want: "content"},
		{name: "ordinal", edit: func(in *ChunkIdentityInput) { in.DuplicateOrdinal = -1 }, want: "duplicate_ordinal"},
		{name: "kind too long", edit: func(in *ChunkIdentityInput) { in.StructureKind = strings.Repeat("a", 65) }, want: "64"},
		{name: "path too long", edit: func(in *ChunkIdentityInput) { in.HeadingPath = strings.Repeat("a", 1025) }, want: "1024"},
		{name: "path control", edit: func(in *ChunkIdentityInput) { in.HeadingPath = "Course / Bad\nPath" }, want: "control"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.edit(&input)
			_, err := StableChunkID(input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("StableChunkID() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestChunkPolicyHashIsDeterministicAndSensitive(t *testing.T) {
	policy := ChunkPolicyIdentity{
		Format: FormatMarkdown, ParserVersion: "markdown-v1", MaxTokens: 160, OverlapLines: 2,
	}
	first, err := ChunkPolicyHash(policy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ChunkPolicyHash(policy)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("policy hashes = %q and %q", first, second)
	}
	policy.MaxTokens++
	changed, err := ChunkPolicyHash(policy)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("changed policy retained the same hash")
	}
}

func TestChunkPolicyHashRejectsInvalidPolicy(t *testing.T) {
	tests := []ChunkPolicyIdentity{
		{},
		{Format: FormatMarkdown, ParserVersion: "v1", MaxTokens: 0},
		{Format: FormatMarkdown, ParserVersion: "v1", MaxTokens: 10, OverlapLines: -1},
		{Format: "pdf", ParserVersion: "v1", MaxTokens: 10},
	}
	for _, policy := range tests {
		if _, err := ChunkPolicyHash(policy); err == nil {
			t.Fatalf("ChunkPolicyHash(%#v) error = nil", policy)
		}
	}
}

func validChunkIdentity(content string) ChunkIdentityInput {
	return ChunkIdentityInput{
		KnowledgeScope:   "document-ingestion-course",
		DocumentID:       "course-markdown",
		StructureKind:    "paragraph",
		HeadingPath:      "Course / Identity",
		Content:          content,
		DuplicateOrdinal: 0,
	}
}
