package promptbudget

import (
	"fmt"
	"strings"
	"text/template"
)

type TemplateData struct {
	System string
	Prompt string
}

// Render executes the model-owned Ollama template with the current system and
// user prompt. The project does not duplicate model-specific wrapper strings.
func Render(templateText string, system string, prompt string) (string, error) {
	tmpl, err := template.New("ollama-prompt").Option("missingkey=error").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("parse Ollama prompt template: %w", err)
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, TemplateData{System: system, Prompt: prompt}); err != nil {
		return "", fmt.Errorf("render Ollama prompt template: %w", err)
	}
	return rendered.String(), nil
}
