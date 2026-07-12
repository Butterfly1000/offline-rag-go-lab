package recentchat

import (
	"bytes"
	"encoding/json"
	"errors"
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
	Model    string             `json:"model"`
	Messages []OllamaMessage    `json:"messages"`
	Stream   bool               `json:"stream"`
	Format   json.RawMessage    `json:"format,omitempty"`
	Options  *OllamaChatOptions `json:"options,omitempty"`
}

type OllamaChatOptions struct {
	NumPredict  int      `json:"num_predict"`
	Temperature *float64 `json:"temperature,omitempty"`
}

type OllamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Content string `json:"-"`
}

type OllamaShowRequest struct {
	Model string `json:"model"`
}

type OllamaModelSummary struct {
	Model             string
	Family            string
	Architecture      string
	ParameterSize     string
	QuantizationLevel string
	ContextLength     int
	Template          string
	Parameters        string
	Capabilities      []string
}

type ollamaShowResponse struct {
	Template     string   `json:"template"`
	Parameters   string   `json:"parameters"`
	Capabilities []string `json:"capabilities"`
	Details      struct {
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
	ModelInfo map[string]json.RawMessage `json:"model_info"`
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

func (c *HTTPOllamaClient) GenerateText(model, system, prompt string, maxTokens int) (string, error) {
	response, err := c.Chat(OllamaChatRequest{
		Model: model,
		Messages: []OllamaMessage{
			{Role: RoleSystem, Content: system},
			{Role: RoleUser, Content: prompt},
		},
		Stream:  false,
		Options: &OllamaChatOptions{NumPredict: maxTokens},
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

// GenerateJSON asks Ollama to constrain the model response with a JSON schema.
// Callers must still validate the decoded facts because schema only controls shape.
func (c *HTTPOllamaClient) GenerateJSON(model, system, prompt string, schema json.RawMessage, maxTokens int) ([]byte, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("Ollama model is required")
	}
	if strings.TrimSpace(system) == "" {
		return nil, fmt.Errorf("Ollama system prompt is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("Ollama user prompt is required")
	}
	if len(schema) == 0 || !json.Valid(schema) {
		return nil, fmt.Errorf("Ollama JSON schema must be valid JSON")
	}
	if maxTokens <= 0 {
		return nil, fmt.Errorf("Ollama max output tokens must be positive: %d", maxTokens)
	}

	temperature := 0.0
	response, err := c.Chat(OllamaChatRequest{
		Model: model,
		Messages: []OllamaMessage{
			{Role: RoleSystem, Content: system},
			{Role: RoleUser, Content: prompt},
		},
		Stream:  false,
		Format:  append(json.RawMessage(nil), schema...),
		Options: &OllamaChatOptions{NumPredict: maxTokens, Temperature: &temperature},
	})
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(response.Content)
	if content == "" {
		return nil, fmt.Errorf("Ollama structured response is empty")
	}
	return []byte(content), nil
}

// Show reads model metadata used when constructing a real Ollama request.
// It returns only the fields needed for context-budget and template teaching.
func (c *HTTPOllamaClient) Show(model string) (OllamaModelSummary, error) {
	body, err := json.Marshal(OllamaShowRequest{Model: model})
	if err != nil {
		return OllamaModelSummary{}, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/show", bytes.NewReader(body))
	if err != nil {
		return OllamaModelSummary{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return OllamaModelSummary{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return OllamaModelSummary{}, fmt.Errorf("ollama show failed: status %d", resp.StatusCode)
		}
		return OllamaModelSummary{}, fmt.Errorf("ollama show failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out ollamaShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return OllamaModelSummary{}, err
	}

	architecture, err := rawString(out.ModelInfo["general.architecture"])
	if err != nil {
		return OllamaModelSummary{}, fmt.Errorf("read Ollama model architecture: %w", err)
	}
	contextKey := architecture + ".context_length"
	contextLength, err := rawInt(out.ModelInfo[contextKey])
	if err != nil {
		return OllamaModelSummary{}, fmt.Errorf("read Ollama model context length %q: %w", contextKey, err)
	}
	return OllamaModelSummary{
		Model:             model,
		Family:            out.Details.Family,
		Architecture:      architecture,
		ParameterSize:     out.Details.ParameterSize,
		QuantizationLevel: out.Details.QuantizationLevel,
		ContextLength:     contextLength,
		Template:          out.Template,
		Parameters:        out.Parameters,
		Capabilities:      out.Capabilities,
	}, nil
}

func (c *HTTPOllamaClient) ContextLength(model string) (int, error) {
	summary, err := c.Show(model)
	if err != nil {
		return 0, err
	}
	return summary.ContextLength, nil
}

func rawString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("metadata field is missing")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.New("metadata value is empty")
	}
	return value, nil
}

func rawInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errors.New("metadata field is missing")
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, errors.New("metadata value must be positive")
	}
	return value, nil
}
