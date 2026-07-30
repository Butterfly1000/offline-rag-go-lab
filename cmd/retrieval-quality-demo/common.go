package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/retrievalquality"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

type commandResources struct {
	Dataset    retrievalquality.Dataset
	QdrantURL  string
	OllamaURL  string
	EmbedModel string
	Tokenizer  retrievalquality.Tokenizer
	EncoderID  string
}

func loadCommandResources(configPath, datasetPath string) (commandResources, error) {
	values, err := fileconfig.Load(configPath)
	if err != nil {
		return commandResources{}, err
	}
	required := func(key string) (string, error) {
		return fileconfig.Required(values, key)
	}
	qdrantURL, err := required("QDRANT_BASE_URL")
	if err != nil {
		return commandResources{}, err
	}
	ollamaURL, err := required("OLLAMA_BASE_URL")
	if err != nil {
		return commandResources{}, err
	}
	if err := validateLocalHTTPURL("QDRANT_BASE_URL", qdrantURL); err != nil {
		return commandResources{}, err
	}
	if err := validateLocalHTTPURL("OLLAMA_BASE_URL", ollamaURL); err != nil {
		return commandResources{}, err
	}
	model, err := required("OLLAMA_EMBED_MODEL")
	if err != nil {
		return commandResources{}, err
	}
	tokenizerPath, err := required("RECENT_CHAT_TOKENIZER_PATH")
	if err != nil {
		return commandResources{}, err
	}
	dataset, err := retrievalquality.LoadDataset(datasetPath)
	if err != nil {
		return commandResources{}, err
	}
	counter, err := tokenizerdemo.LoadCounter(tokenizerPath)
	if err != nil {
		return commandResources{}, err
	}
	tokenizer, err := retrievalquality.NewQwenTokenizer(counter)
	if err != nil {
		return commandResources{}, err
	}
	identity, err := tokenizerIdentity(tokenizerPath)
	if err != nil {
		return commandResources{}, err
	}
	return commandResources{
		Dataset: dataset, QdrantURL: qdrantURL, OllamaURL: ollamaURL,
		EmbedModel: model, Tokenizer: tokenizer, EncoderID: identity,
	}, nil
}

func validateLocalHTTPURL(name, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" {
		return fmt.Errorf("%s must be a valid HTTP(S) URL", name)
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("%s must point to this machine", name)
	}
	return nil
}

func tokenizerIdentity(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("tokenizer path is required")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read tokenizer identity: %w", err)
	}
	sum := sha256.Sum256(content)
	return "qwen2:" + hex.EncodeToString(sum[:]) + ":field-bm25-v1", nil
}
