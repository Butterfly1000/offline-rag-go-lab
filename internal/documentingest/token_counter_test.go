package documentingest

import (
	"path/filepath"
	"testing"
)

func TestQwenTokenCounterUsesRepositoryTokenizer(t *testing.T) {
	counter, err := NewQwenTokenCounter(filepath.Join("..", "..", "assets", "tokenizers", "qwen2", "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	count, err := counter.Count("我叫小黄，这个项目是 Go 写的。")
	if err != nil {
		t.Fatal(err)
	}
	if count != 15 {
		t.Fatalf("token count = %d, want 15", count)
	}
}
