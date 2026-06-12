package compression

import (
	"testing"

	world "offline-rag-go-lab/internal/gateway/level1_world"
)

func TestSimpleCompressorDedupesLimitsAndTruncates(t *testing.T) {
	compressor := SimpleCompressor{}
	hits := []world.RetrievalHit{
		{
			KnowledgeChunk: world.KnowledgeChunk{ChunkID: "a#0", Title: "FAQ", Text: "重复知识片段重复知识片段重复知识片段"},
			Score:          0.9,
		},
		{
			KnowledgeChunk: world.KnowledgeChunk{ChunkID: "b#0", Title: "FAQ", Text: "重复知识片段重复知识片段重复知识片段"},
			Score:          0.8,
		},
		{
			KnowledgeChunk: world.KnowledgeChunk{ChunkID: "c#0", Title: "FAQ", Text: "另一条命中内容"},
			Score:          0.7,
		},
	}

	out := compressor.Compress(hits, 1, 8)
	if len(out) != 1 {
		t.Fatalf("expected 1 compressed hit, got %d", len(out))
	}
	if out[0].ChunkID != "a#0" {
		t.Fatalf("expected highest-ranked duplicate to remain, got %s", out[0].ChunkID)
	}
	if len([]rune(out[0].Text)) > 8 {
		t.Fatalf("expected compressed text to be truncated, got %s", out[0].Text)
	}
}
