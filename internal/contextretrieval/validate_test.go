package contextretrieval

import (
	"math"
	"strings"
	"testing"
)

func TestValidateHitEnforcesSourceOwnership(t *testing.T) {
	tests := []struct {
		name string
		hit  Hit
	}{
		{
			name: "memory",
			hit: Hit{
				Source: SourceMemory, ID: " memory:7 ", Content: " project_fact/language: Go ",
				Score: 0.91, UserID: " u-001 ", Kind: " project_fact ",
			},
		},
		{
			name: "document",
			hit: Hit{
				Source: SourceDocument, ID: " document:chunk-1 ", Content: " 项目使用 Go。 ",
				Score: 0.87, KnowledgeScope: " offline-rag-course ", Title: " 项目说明 ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateHit(tt.hit)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != strings.TrimSpace(tt.hit.ID) || got.Content != strings.TrimSpace(tt.hit.Content) {
				t.Fatalf("ValidateHit() = %#v", got)
			}
			switch got.Source {
			case SourceMemory:
				if got.UserID != "u-001" || got.KnowledgeScope != "" {
					t.Fatalf("validated memory hit = %#v", got)
				}
			case SourceDocument:
				if got.KnowledgeScope != "offline-rag-course" || got.UserID != "" {
					t.Fatalf("validated document hit = %#v", got)
				}
			}
		})
	}
}

func TestValidateHitRejectsInvalidInput(t *testing.T) {
	validMemory := Hit{
		Source: SourceMemory, ID: "memory:7", Content: "project_fact/language: Go",
		Score: 0.91, UserID: "u-001",
	}
	validDocument := Hit{
		Source: SourceDocument, ID: "document:chunk-1", Content: "项目使用 Go。",
		Score: 0.87, KnowledgeScope: "offline-rag-course",
	}

	tests := []struct {
		name string
		hit  Hit
		want string
	}{
		{name: "empty ID", hit: replaceHit(validMemory, func(h *Hit) { h.ID = " " }), want: "ID is required"},
		{name: "empty content", hit: replaceHit(validMemory, func(h *Hit) { h.Content = " " }), want: "content is required"},
		{name: "NaN score", hit: replaceHit(validMemory, func(h *Hit) { h.Score = math.NaN() }), want: "score must be finite"},
		{name: "infinite score", hit: replaceHit(validMemory, func(h *Hit) { h.Score = math.Inf(1) }), want: "score must be finite"},
		{name: "unknown source", hit: replaceHit(validMemory, func(h *Hit) { h.Source = "cache" }), want: "unknown retrieval source"},
		{name: "memory missing user", hit: replaceHit(validMemory, func(h *Hit) { h.UserID = "" }), want: "memory hit user_id is required"},
		{name: "memory carries scope", hit: replaceHit(validMemory, func(h *Hit) { h.KnowledgeScope = "scope-a" }), want: "memory hit must not carry knowledge_scope"},
		{name: "document missing scope", hit: replaceHit(validDocument, func(h *Hit) { h.KnowledgeScope = "" }), want: "document hit knowledge_scope is required"},
		{name: "document carries user", hit: replaceHit(validDocument, func(h *Hit) { h.UserID = "u-001" }), want: "document hit must not carry user_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateHit(tt.hit)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateHit() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateHitCopiesMetadata(t *testing.T) {
	input := Hit{
		Source: SourceMemory, ID: "memory:7", Content: "Go", Score: 0.9,
		UserID: "u-001", Metadata: map[string]string{" key ": " value "},
	}
	got, err := ValidateHit(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata["key"] != "value" {
		t.Fatalf("metadata = %#v", got.Metadata)
	}

	input.Metadata["key"] = "changed"
	if got.Metadata["key"] != "value" {
		t.Fatalf("validated metadata aliases input: %#v", got.Metadata)
	}
}

func replaceHit(hit Hit, replace func(*Hit)) Hit {
	replace(&hit)
	return hit
}
