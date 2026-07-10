package tokenizerdemo

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TokenizerSummary contains only the top-level structure needed for teaching
// and compatibility diagnosis. It deliberately excludes the full vocabulary.
type TokenizerSummary struct {
	Version           string
	ModelType         string
	NormalizerType    string
	PreTokenizerType  string
	PostProcessorType string
	DecoderType       string
	VocabSize         int
	AddedTokens       int
	SHA256            string
}

type componentConfig struct {
	Type string `json:"type"`
}

type modelConfig struct {
	Type  string          `json:"type"`
	Vocab json.RawMessage `json:"vocab"`
}

type tokenizerConfig struct {
	Version       string            `json:"version"`
	AddedTokens   []json.RawMessage `json:"added_tokens"`
	Normalizer    componentConfig   `json:"normalizer"`
	PreTokenizer  componentConfig   `json:"pre_tokenizer"`
	PostProcessor componentConfig   `json:"post_processor"`
	Decoder       componentConfig   `json:"decoder"`
	Model         modelConfig       `json:"model"`
}

// InspectFile reads tokenizer.json as configuration data. It does not execute
// normalization, pre-tokenization, model merges, or post-processing rules.
func InspectFile(path string) (TokenizerSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TokenizerSummary{}, fmt.Errorf("read tokenizer config %q: %w", path, err)
	}

	var config tokenizerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return TokenizerSummary{}, fmt.Errorf("decode tokenizer config %q: %w", path, err)
	}

	vocabSize, err := countJSONCollection(config.Model.Vocab)
	if err != nil {
		return TokenizerSummary{}, fmt.Errorf("count model vocab in %q: %w", path, err)
	}

	return TokenizerSummary{
		Version:           config.Version,
		ModelType:         config.Model.Type,
		NormalizerType:    config.Normalizer.Type,
		PreTokenizerType:  config.PreTokenizer.Type,
		PostProcessorType: config.PostProcessor.Type,
		DecoderType:       config.Decoder.Type,
		VocabSize:         vocabSize,
		AddedTokens:       len(config.AddedTokens),
		SHA256:            fmt.Sprintf("%x", sha256.Sum256(data)),
	}, nil
}

// VerifySHA256 compares a recorded fingerprint with the file fingerprint.
// It proves byte-for-byte consistency, not compatibility with a model.
func VerifySHA256(actual string, expected string) error {
	actual = strings.TrimSpace(actual)
	expected = strings.TrimSpace(expected)
	if strings.EqualFold(actual, expected) {
		return nil
	}
	return fmt.Errorf("tokenizer SHA256 mismatch: expected %s, actual %s", expected, actual)
}

// countJSONCollection supports object vocabularies used by BPE/WordPiece and
// array vocabularies used by tokenizer formats such as Unigram.
func countJSONCollection(raw json.RawMessage) (int, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}

	switch raw[0] {
	case '{':
		var values map[string]json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return 0, err
		}
		return len(values), nil
	case '[':
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return 0, err
		}
		return len(values), nil
	default:
		return 0, fmt.Errorf("vocab must be a JSON object or array")
	}
}
