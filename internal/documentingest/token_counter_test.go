package documentingest

import (
	"path/filepath"
	"testing"

	"offline-rag-go-lab/internal/tokenizerdemo"
)

func TestQwenTokenCounterUsesRepositoryTokenizer(t *testing.T) {
	path := filepath.Join("..", "..", "assets", "tokenizers", "qwen2", "tokenizer.json")
	summary, err := tokenizerdemo.InspectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := tokenizerdemo.VerifySHA256(summary.SHA256, "f7c9b2dba4a296b1aa76c16a34b8225c0c118978400d4bb66bff0902d702f5b8"); err != nil {
		t.Fatalf("%v; document token-count golden values are tied to this exact tokenizer asset", err)
	}

	counter, err := NewQwenTokenCounter(path)
	if err != nil {
		t.Fatal(err)
	}
	count, err := counter.Count("我叫小黄，这个项目是 Go 写的。")
	if err != nil {
		t.Fatal(err)
	}
	if count != 13 {
		t.Fatalf("token count = %d, want 13", count)
	}
}
