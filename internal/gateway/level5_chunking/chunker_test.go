package chunking

import (
	"strings"
	"testing"

	world "offline-rag-go-lab/internal/gateway/level1_world"
)

func TestBuildChunksUsesHeadingAsSectionTitle(t *testing.T) {
	chunks, err := BuildChunks(world.IngestRequest{
		DocumentID: "policy",
		Title:      "退款政策",
		SourceRef:  "policy.md",
		Text:       "# 申请条件\n用户在购买后 7 天内可以申请退款。",
	})
	if err != nil {
		t.Fatalf("BuildChunks returned error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Title != "退款政策 / 申请条件" {
		t.Fatalf("expected chunk title to include section heading, got %s", chunks[0].Title)
	}
}

func TestBuildChunksSplitsLongLineWithStableIDs(t *testing.T) {
	longLine := strings.Repeat("很长的说明文本", 20)
	chunks, err := BuildChunks(world.IngestRequest{
		DocumentID: "guide",
		Title:      "使用说明",
		SourceRef:  "guide.md",
		Text:       longLine,
	})
	if err != nil {
		t.Fatalf("BuildChunks returned error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected long line to split into multiple chunks, got %d", len(chunks))
	}
	if chunks[0].ChunkID != "guide#0" || chunks[1].ChunkID != "guide#1" {
		t.Fatalf("expected stable chunk ids, got %s and %s", chunks[0].ChunkID, chunks[1].ChunkID)
	}
	if runeCount(chunks[1].Text) >= runeCount(longLine) {
		t.Fatalf("expected split chunk to be smaller than original line")
	}
}
