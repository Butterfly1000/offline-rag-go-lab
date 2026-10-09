package tokenizerdemo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/model/wordlevel"
)

type qwenTokenizerGolden struct {
	sha256        string
	longTextCount int
	singleWoID    int
	separatedIDs  []int
}

var qwenTokenizerGoldens = map[string]qwenTokenizerGolden{
	"f7c9b2dba4a296b1aa76c16a34b8225c0c118978400d4bb66bff0902d702f5b8": {
		sha256:        "f7c9b2dba4a296b1aa76c16a34b8225c0c118978400d4bb66bff0902d702f5b8",
		longTextCount: 31,
		singleWoID:    35946,
		separatedIDs:  []int{35946, 64, 38342},
	},
	"b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8": {
		sha256:        "b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8",
		longTextCount: 41,
		singleWoID:    56023,
		separatedIDs:  []int{56023, 87, 73306},
	},
}

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
	golden := assertKnownQwenTokenizerAsset(t)
	counter, err := LoadCounter(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	text := "版本从 pending 进入 building。构建成功后进入 ready，发布后才成为 active；构建失败则进入 failed 并允许重试。"
	count, tokens, ids, err := counter.CountText(text)
	if err != nil {
		t.Fatal(err)
	}
	if count != golden.longTextCount || len(tokens) != count || len(ids) != count {
		t.Fatalf("CountText() count=%d tokens=%d ids=%d, want %d for tokenizer %s", count, len(tokens), len(ids), golden.longTextCount, golden.sha256)
	}
}

func TestQwenCounterCountsSingleChineseAddedToken(t *testing.T) {
	golden := assertKnownQwenTokenizerAsset(t)
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
	if ids[0] != golden.singleWoID {
		t.Fatalf("CountText(我) ids=%v, want [%d] for tokenizer %s", ids, golden.singleWoID, golden.sha256)
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

func TestQwenCounterKeepsSeparatedChineseTokens(t *testing.T) {
	golden := assertKnownQwenTokenizerAsset(t)
	counter, err := LoadCounter(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	count, tokens, ids, err := counter.CountText("我a未")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || len(tokens) != 3 || len(ids) != 3 {
		t.Fatalf("CountText(我a未) count=%d tokens=%d ids=%d, want three tokens", count, len(tokens), len(ids))
	}
	if !reflect.DeepEqual(ids, golden.separatedIDs) {
		t.Fatalf("CountText(我a未) ids=%v, want %v for tokenizer %s", ids, golden.separatedIDs, golden.sha256)
	}
}

func qwenTokenizerTestPath() string {
	if value := os.Getenv("QWEN_TOKENIZER_PATH"); value != "" {
		return value
	}
	return filepath.Join("..", "..", "assets", "tokenizers", "qwen2", "tokenizer.json")
}

func assertKnownQwenTokenizerAsset(t *testing.T) qwenTokenizerGolden {
	t.Helper()

	summary, err := InspectFile(qwenTokenizerTestPath())
	if err != nil {
		t.Fatal(err)
	}
	golden, ok := qwenTokenizerGoldens[summary.SHA256]
	if !ok {
		t.Fatalf("unknown tokenizer SHA256 %s; token-count golden values must be reviewed before accepting a new tokenizer asset", summary.SHA256)
	}
	return golden
}
