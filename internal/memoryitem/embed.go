package memoryitem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const maxHTTPErrorBodyBytes = 2048

type Embedder interface {
	Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}

type HTTPOllamaEmbedder struct {
	baseURL string
	client  *http.Client
}

func NewHTTPOllamaEmbedder(baseURL string) *HTTPOllamaEmbedder {
	return &HTTPOllamaEmbedder{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *HTTPOllamaEmbedder) Embed(ctx context.Context, model string, texts []string) ([][]float32, error) {
	if c == nil || c.client == nil || c.baseURL == "" {
		return nil, fmt.Errorf("Ollama embed HTTP client and base URL are required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("Ollama embedding model is required")
	}
	if len(texts) == 0 {
		return nil, fmt.Errorf("Ollama embedding texts are required")
	}
	input := append([]string(nil), texts...)
	for index, text := range input {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("Ollama embedding text %d is empty", index)
		}
	}

	body, err := json.Marshal(struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("encode Ollama embed request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Ollama embed request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Ollama embed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, httpStatusError("Ollama embed", response)
	}

	var decoded struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode Ollama embed response: %w", err)
	}
	if err := validateEmbeddingVectors(decoded.Embeddings, len(input)); err != nil {
		return nil, fmt.Errorf("validate Ollama embeddings: %w", err)
	}
	return decoded.Embeddings, nil
}

func validateEmbeddingVectors(vectors [][]float32, expectedCount int) error {
	if expectedCount <= 0 {
		return fmt.Errorf("expected embedding count must be positive: %d", expectedCount)
	}
	if len(vectors) != expectedCount {
		return fmt.Errorf("embedding count is %d, want %d", len(vectors), expectedCount)
	}
	dimension := 0
	for vectorIndex, vector := range vectors {
		if len(vector) == 0 {
			return fmt.Errorf("embedding %d is empty", vectorIndex)
		}
		if dimension == 0 {
			dimension = len(vector)
		} else if len(vector) != dimension {
			return fmt.Errorf("embedding %d dimension is %d, want %d", vectorIndex, len(vector), dimension)
		}
		for valueIndex, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return fmt.Errorf("embedding %d value %d is not finite", vectorIndex, valueIndex)
			}
		}
	}
	return nil
}

func httpStatusError(service string, response *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxHTTPErrorBodyBytes+1))
	if readErr != nil {
		return fmt.Errorf("%s failed: status %d", service, response.StatusCode)
	}
	truncated := len(body) > maxHTTPErrorBodyBytes
	if truncated {
		body = body[:maxHTTPErrorBodyBytes]
	}
	detail := strings.TrimSpace(string(body))
	if truncated {
		detail += " ...[truncated]"
	}
	if detail == "" {
		return fmt.Errorf("%s failed: status %d", service, response.StatusCode)
	}
	return fmt.Errorf("%s failed: status %d: %s", service, response.StatusCode, detail)
}

var _ Embedder = (*HTTPOllamaEmbedder)(nil)
