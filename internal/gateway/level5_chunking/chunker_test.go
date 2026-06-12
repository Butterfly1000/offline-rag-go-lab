package chunking

import (
	"strings" // TestBuildChunksSplitsLongLineWithStableIDs 里 Repeat 生成长文本
	"testing"

	world "offline-rag-go-lab/internal/gateway/level1_world"
)

// TestBuildChunksUsesHeadingAsSectionTitle 验证 Markdown # 标题会并入 chunk.Title。
func TestBuildChunksUsesHeadingAsSectionTitle(t *testing.T) {
	chunks, err := BuildChunks(world.IngestRequest{
		DocumentID: "policy",
		Title:      "退款政策",
		SourceRef:  "policy.md",
		Text:       "# 申请条件\n用户在购买后 7 天内可以申请退款。",
	})
	if err != nil {
		t.Fatalf("BuildChunks returned error: %v", err) // 失败则终止整个测试
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Title != "退款政策 / 申请条件" {
		t.Fatalf("expected chunk title to include section heading, got %s", chunks[0].Title)
	}
}

// TestBuildChunksSplitsLongLineWithStableIDs 验证超长单行会被切成多块且 ID 连续。
func TestBuildChunksSplitsLongLineWithStableIDs(t *testing.T) {
	longLine := strings.Repeat("很长的说明文本", 20) // 重复 20 次，超过 defaultChunkMaxChars
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
