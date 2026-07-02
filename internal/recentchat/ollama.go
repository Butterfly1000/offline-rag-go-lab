package recentchat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaMessage struct {
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
}

type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Content string `json:"-"`
}

type OllamaClient interface {
	Chat(req OllamaChatRequest) (OllamaChatResponse, error)
}

type HTTPOllamaClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPOllamaClient(baseURL string) *HTTPOllamaClient {
	return &HTTPOllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *HTTPOllamaClient) Chat(req OllamaChatRequest) (OllamaChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return OllamaChatResponse{}, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return OllamaChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return OllamaChatResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return OllamaChatResponse{}, fmt.Errorf("ollama chat failed: status %d", resp.StatusCode)
		}
		return OllamaChatResponse{}, fmt.Errorf("ollama chat failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return OllamaChatResponse{}, err
	}
	out.Content = out.Message.Content

	return out, nil
}
