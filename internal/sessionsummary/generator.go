package sessionsummary

import (
	"fmt"
	"strings"
)

type TextGenerator interface {
	GenerateText(model, system, prompt string, maxTokens int) (string, error)
}

type Generator struct {
	client TextGenerator
}

func NewGenerator(client TextGenerator) Generator {
	return Generator{client: client}
}

func (g Generator) Update(model, previous string, messages []SourceMessage, maxTokens int) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("text generator is required")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("model is required")
	}
	if maxTokens <= 0 {
		return "", fmt.Errorf("maximum output tokens must be positive: %d", maxTokens)
	}
	prompt, err := BuildUpdatePrompt(previous, messages)
	if err != nil {
		return "", err
	}

	generated, err := g.client.GenerateText(model, SummarySystemPrompt, prompt, maxTokens)
	if err != nil {
		return "", fmt.Errorf("generate summary: %w", err)
	}
	generated = strings.TrimSpace(generated)
	generated = strings.TrimPrefix(generated, "<updated_summary>")
	generated = strings.TrimSuffix(generated, "</updated_summary>")
	generated = strings.TrimSpace(generated)
	if generated == "" {
		return "", fmt.Errorf("generate summary: model returned empty summary")
	}
	return generated, nil
}
