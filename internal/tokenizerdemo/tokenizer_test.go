package tokenizerdemo

import (
	"os"
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

func TestQwenCounterDoesNotDropChineseAroundBackreferencePattern(t *testing.T) {
	counter, err := LoadCounter(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	text := "版本从 pending 进入 building。构建成功后进入 ready，发布后才成为 active；构建失败则进入 failed 并允许重试。"
	count, tokens, ids, err := counter.CountText(text)
	if err != nil {
		t.Fatal(err)
	}
	if count != 41 || len(tokens) != count || len(ids) != count {
		t.Fatalf("CountText() count=%d tokens=%d ids=%d, want 41", count, len(tokens), len(ids))
	}
}

func TestQwenCounterCountsSingleChineseAddedToken(t *testing.T) {
	counter, err := LoadCounter(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	count, tokens, ids, err := counter.CountText("我")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := counter.tokenizer.EncodeSingleSequence(tokenizer.NewInputSequence("我"), 0, tokenizer.Byte)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(tokens) != 1 || len(ids) != 1 {
		id, found := counter.tokenizer.TokenToId("我")
		t.Fatalf("CountText(我) count=%d tokens=%v ids=%v raw=%v token_id=%d found=%t, want one added token", count, tokens, ids, raw.Tokens, id, found)
	}
}

func TestAddedVocabularySelectsLeftmostLongestMatches(t *testing.T) {
	model, err := wordlevel.New(map[string]int{"<unk>": 0}, "<unk>")
	if err != nil {
		t.Fatal(err)
	}
	tk := tokenizer.NewTokenizer(model)
	tk.AddSpecialTokens([]tokenizer.AddedToken{
		tokenizer.NewAddedToken("a", true),
		tokenizer.NewAddedToken("ab", true),
		tokenizer.NewAddedToken("b", true),
	})
	encoding, err := tk.EncodeSingle("ab", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoding.Tokens) != 1 || encoding.Tokens[0] != "ab" {
		t.Fatalf("tokens = %#v, want longest leftmost [ab]", encoding.Tokens)
	}
}

func TestQwenCounterKeepsSeparatedChineseAddedTokens(t *testing.T) {
	counter, err := LoadCounter(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	_, tokens, _, err := counter.CountText("我a未")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 || tokens[0] != "我" || tokens[1] != "a" || tokens[2] != "未" {
		t.Fatalf("tokens = %#v, want [我 a 未]", tokens)
	}
}

func qwenTokenizerTestPath() string {
	if value := os.Getenv("QWEN_TOKENIZER_PATH"); value != "" {
		return value
	}
	return filepath.Join("..", "..", "assets", "tokenizers", "qwen2", "tokenizer.json")
}
