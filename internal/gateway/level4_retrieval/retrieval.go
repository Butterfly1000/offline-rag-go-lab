package retrieval

import (
	"math"
	"regexp"
	"strings"
)

var asciiTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func NormalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func Similarity(query, text string) float64 {
	qTokens := tokenize(query)
	tTokens := tokenize(text)
	if len(qTokens) == 0 || len(tTokens) == 0 {
		return 0
	}

	overlap := 0
	for token := range qTokens {
		if _, ok := tTokens[token]; ok {
			overlap++
		}
	}
	if overlap == 0 {
		return 0
	}

	return float64(overlap) / math.Sqrt(float64(len(qTokens)*len(tTokens)))
}

func tokenize(value string) map[string]struct{} {
	value = NormalizeText(value)
	result := make(map[string]struct{})

	for _, match := range asciiTokenPattern.FindAllString(value, -1) {
		result[match] = struct{}{}
	}

	cjk := make([]rune, 0, len(value))
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			cjk = append(cjk, r)
			result[string(r)] = struct{}{}
		}
	}
	for i := 0; i < len(cjk)-1; i++ {
		result[string(cjk[i:i+2])] = struct{}{}
	}
	return result
}
