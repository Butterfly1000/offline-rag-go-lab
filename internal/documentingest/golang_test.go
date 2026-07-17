package documentingest

import (
	"strings"
	"testing"
)

func TestGoSourceChunkingKeepsDocCommentWithDeclaration(t *testing.T) {
	document := normalizedTestDocument(t, FormatGo, `// Package course teaches ingestion.
package course

import "fmt"

// Service stores one name.
type Service struct {
	Name string
}

// Print returns the service name.
func (s Service) Print() string {
	return fmt.Sprint(s.Name)
}`)
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 500}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 4 {
		t.Fatalf("chunks = %#v, want preamble/import/type/function", chunks)
	}
	var typeChunk, functionChunk *Chunk
	for index := range chunks {
		switch chunks[index].HeadingPath {
		case "package course / type Service":
			typeChunk = &chunks[index]
		case "package course / func Service.Print":
			functionChunk = &chunks[index]
		}
	}
	if typeChunk == nil || !strings.HasPrefix(typeChunk.Text, "// Service stores one name.\ntype Service") {
		t.Fatalf("type chunk = %#v", typeChunk)
	}
	if functionChunk == nil || !strings.HasPrefix(functionChunk.Text, "// Print returns the service name.\nfunc") {
		t.Fatalf("function chunk = %#v", functionChunk)
	}
}

func TestGoSourceChunkingKeepsDetachedAndTrailingComments(t *testing.T) {
	document := normalizedTestDocument(t, FormatGo, `package course

var First = 1

// This operational note is intentionally detached from Second.

var Second = 2

// This trailing note must remain searchable.
`)
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 500}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, chunk := range chunks {
		joined += "\n" + chunk.Text
	}
	for _, comment := range []string{
		"// This operational note is intentionally detached from Second.",
		"// This trailing note must remain searchable.",
	} {
		if strings.Count(joined, comment) != 1 {
			t.Fatalf("comment %q count = %d in chunks %#v, want exactly one", comment, strings.Count(joined, comment), chunks)
		}
	}
}

func TestGoSourceChunkingSplitsOversizedDeclarationByLinesWithOverlap(t *testing.T) {
	document := normalizedTestDocument(t, FormatGo, `package course

func Steps() {
	stepOne()
	stepTwo()
	stepThree()
	stepFour()
}`)
	chunks, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 34, OverlapLines: 1}, runeTokenCounter{})
	if err != nil {
		t.Fatal(err)
	}
	var declarations []Chunk
	for _, chunk := range chunks {
		if chunk.StructureKind == "go_declaration" {
			declarations = append(declarations, chunk)
		}
		if chunk.TokenCount > 34 {
			t.Fatalf("chunk exceeds limit: %#v", chunk)
		}
	}
	if len(declarations) < 2 {
		t.Fatalf("declarations = %#v, want split declaration", declarations)
	}
	firstLines := strings.Split(declarations[0].Text, "\n")
	secondLines := strings.Split(declarations[1].Text, "\n")
	if firstLines[len(firstLines)-1] != secondLines[0] {
		t.Fatalf("overlap = %q / %q", declarations[0].Text, declarations[1].Text)
	}
}

func TestGoSourceChunkingRejectsMalformedGo(t *testing.T) {
	document := normalizedTestDocument(t, FormatGo, "package course\n\nfunc Broken( {")
	_, err := ChunkDocument(document, ChunkPolicy{MaxTokens: 100}, runeTokenCounter{})
	if err == nil || !strings.Contains(err.Error(), "parse Go source") {
		t.Fatalf("ChunkDocument() error = %v", err)
	}
}
