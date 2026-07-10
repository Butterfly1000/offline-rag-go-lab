package promptbudget

import "fmt"

type TextTokenCounter interface {
	CountText(text string) (count int, tokens []string, ids []int, err error)
}

type TokenComparison struct {
	SystemTokens     int
	PromptTokens     int
	ContentTokens    int
	RenderedTokens   int
	TemplateOverhead int
}

// CompareTokens shows the difference between counting message content alone
// and counting the complete prompt after the model template is rendered.
func CompareTokens(counter TextTokenCounter, system string, prompt string, rendered string) (TokenComparison, error) {
	systemTokens, err := countPart(counter, "system content", system)
	if err != nil {
		return TokenComparison{}, err
	}
	promptTokens, err := countPart(counter, "user prompt", prompt)
	if err != nil {
		return TokenComparison{}, err
	}
	renderedTokens, err := countPart(counter, "rendered prompt", rendered)
	if err != nil {
		return TokenComparison{}, err
	}

	contentTokens := systemTokens + promptTokens
	return TokenComparison{
		SystemTokens:     systemTokens,
		PromptTokens:     promptTokens,
		ContentTokens:    contentTokens,
		RenderedTokens:   renderedTokens,
		TemplateOverhead: renderedTokens - contentTokens,
	}, nil
}

func countPart(counter TextTokenCounter, name string, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	count, _, _, err := counter.CountText(text)
	if err != nil {
		return 0, fmt.Errorf("count %s tokens: %w", name, err)
	}
	return count, nil
}
