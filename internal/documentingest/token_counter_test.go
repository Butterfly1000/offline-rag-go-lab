package documentingest

import (
	"path/filepath"
	"testing"

	"offline-rag-go-lab/internal/tokenizerdemo"
)

var qwenTokenizerCountGoldens = map[string]int{
	"f7c9b2dba4a296b1aa76c16a34b8225c0c118978400d4bb66bff0902d702f5b8": 13,
	"b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8": 15,
}

func TestQwenTokenCounterUsesRepositoryTokenizer(t *testing.T) {
	path := filepath.Join("..", "..", "assets", "tokenizers", "qwen2", "tokenizer.json")
	summary, err := tokenizerdemo.InspectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantCount, ok := qwenTokenizerCountGoldens[summary.SHA256]
	if !ok {
		t.Fatalf("unknown tokenizer SHA256 %s; document token-count golden values must be reviewed before accepting a new tokenizer asset", summary.SHA256)
	}

	counter, err := NewQwenTokenCounter(path)
	if err != nil {
		t.Fatal(err)
	}
	count, err := counter.Count("我叫小黄，这个项目是 Go 写的。")
	if err != nil {
		t.Fatal(err)
	}
	if count != wantCount {
		t.Fatalf("token count = %d, want %d for tokenizer %s", count, wantCount, summary.SHA256)
	}
}
