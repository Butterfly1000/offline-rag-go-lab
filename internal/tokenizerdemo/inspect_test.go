package tokenizerdemo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectFileReturnsTokenizerComponentSummary(t *testing.T) {
	path := writeTokenizerFixture(t, `{
  "version": "1.0",
  "added_tokens": [{"id": 3}, {"id": 4}],
  "normalizer": {"type": "NFKC"},
  "pre_tokenizer": {"type": "Sequence"},
  "post_processor": {"type": "TemplateProcessing"},
  "decoder": {"type": "ByteLevel"},
  "model": {
    "type": "BPE",
    "vocab": {"hello": 0, "world": 1, "!": 2}
  }
}`)

	summary, err := InspectFile(path)
	if err != nil {
		t.Fatalf("InspectFile() error = %v", err)
	}

	want := TokenizerSummary{
		Version:           "1.0",
		ModelType:         "BPE",
		NormalizerType:    "NFKC",
		PreTokenizerType:  "Sequence",
		PostProcessorType: "TemplateProcessing",
		DecoderType:       "ByteLevel",
		VocabSize:         3,
		AddedTokens:       2,
	}
	if summary != want {
		t.Fatalf("InspectFile() = %+v, want %+v", summary, want)
	}
}

func TestInspectFileRejectsInvalidJSON(t *testing.T) {
	path := writeTokenizerFixture(t, `{not-json}`)

	_, err := InspectFile(path)
	if err == nil {
		t.Fatal("InspectFile() error = nil, want invalid JSON error")
	}
}

func TestInspectFileCountsArrayVocabulary(t *testing.T) {
	path := writeTokenizerFixture(t, `{
  "version": "1.0",
  "model": {
    "type": "Unigram",
    "vocab": [["<unk>", 0.0], ["hello", -1.2]]
  }
}`)

	summary, err := InspectFile(path)
	if err != nil {
		t.Fatalf("InspectFile() error = %v", err)
	}
	if summary.VocabSize != 2 {
		t.Fatalf("InspectFile() VocabSize = %d, want 2", summary.VocabSize)
	}
}

func writeTokenizerFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "tokenizer.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}
