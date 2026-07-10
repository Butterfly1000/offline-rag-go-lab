package tokenizerdemo

import (
	"os"
	"path/filepath"
	"strings"
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
		SHA256:            "18ad5f36eb651e44f9f01e8ae16975b01e5cbf469ad822b59180a79c11cf09e4",
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

func TestInspectFileReturnsFileSHA256(t *testing.T) {
	path := writeTokenizerFixture(t, `{}`)

	summary, err := InspectFile(path)
	if err != nil {
		t.Fatalf("InspectFile() error = %v", err)
	}
	const want = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
	if summary.SHA256 != want {
		t.Fatalf("InspectFile() SHA256 = %q, want %q", summary.SHA256, want)
	}
}

func TestVerifySHA256AcceptsMatchingFingerprint(t *testing.T) {
	const fingerprint = "ABC123"

	if err := VerifySHA256("abc123", fingerprint); err != nil {
		t.Fatalf("VerifySHA256() error = %v", err)
	}
}

func TestVerifySHA256RejectsDifferentFingerprint(t *testing.T) {
	err := VerifySHA256("actual", "expected")
	if err == nil {
		t.Fatal("VerifySHA256() error = nil, want mismatch error")
	}
	if !strings.Contains(err.Error(), "expected") || !strings.Contains(err.Error(), "actual") {
		t.Fatalf("VerifySHA256() error = %q, want expected and actual fingerprints", err)
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
