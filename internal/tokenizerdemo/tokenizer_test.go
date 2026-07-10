package tokenizerdemo

import (
	"path/filepath"
	"testing"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/model/wordlevel"
)

func TestLoadCounterFailsWhenTokenizerFileDoesNotExist(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "tokenizer.json")

	_, err := LoadCounter(missingPath)
	if err == nil {
		t.Fatal("LoadCounter() error = nil, want missing file error")
	}
}

func TestCounterUsesLoadedTokenizerToCountText(t *testing.T) {
	model, err := wordlevel.New(map[string]int{
		"<unk>": 0,
		"hello": 1,
	}, "<unk>")
	if err != nil {
		t.Fatalf("wordlevel.New() error = %v", err)
	}

	counter := newCounter(tokenizer.NewTokenizer(model))
	count, tokens, ids, err := counter.CountText("hello")
	if err != nil {
		t.Fatalf("CountText() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountText() count = %d, want 1", count)
	}
	if len(tokens) != 1 || tokens[0] != "hello" {
		t.Fatalf("CountText() tokens = %v, want [hello]", tokens)
	}
	if len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("CountText() ids = %v, want [1]", ids)
	}
}
