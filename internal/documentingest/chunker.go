package documentingest

import (
	"fmt"
	"strings"
)

type ChunkPolicy struct {
	MaxTokens    int
	OverlapLines int
}

// Chunk is ready for a manifest and embedding call. Ordinal is presentation
// order; it deliberately does not participate in the stable identity.
type Chunk struct {
	ChunkID       string
	StructureKind string
	HeadingPath   string
	Text          string
	ContentHash   string
	Ordinal       int
	TokenCount    int
}

type structuralUnit struct {
	Kind        string
	HeadingPath string
	Text        string
}

func ChunkDocument(input Document, policy ChunkPolicy, counter TokenCounter) ([]Chunk, error) {
	document, err := NormalizeDocument(input)
	if err != nil {
		return nil, err
	}
	if err := validateChunkPolicy(policy); err != nil {
		return nil, err
	}
	if counter == nil {
		return nil, fmt.Errorf("token counter is required")
	}

	var units []structuralUnit
	switch document.Format {
	case FormatMarkdown:
		units, err = parseMarkdown(string(document.Content))
	case FormatGo:
		units, err = parseGoSource(document.SourceRef, string(document.Content))
	default:
		return nil, fmt.Errorf("unsupported document format %q", document.Format)
	}
	if err != nil {
		return nil, err
	}

	var split []structuralUnit
	for _, unit := range units {
		parts, splitErr := splitStructuralUnit(unit, policy, counter)
		if splitErr != nil {
			return nil, splitErr
		}
		split = append(split, parts...)
	}
	packed, err := packCompatibleUnits(split, policy.MaxTokens, counter)
	if err != nil {
		return nil, err
	}
	if len(packed) == 0 {
		return nil, fmt.Errorf("document produced no chunks")
	}

	duplicates := make(map[string]int)
	chunks := make([]Chunk, 0, len(packed))
	for ordinal, unit := range packed {
		text := normalizePortableText(unit.Text)
		tokenCount, countErr := countTokens(counter, text)
		if countErr != nil {
			return nil, fmt.Errorf("count chunk %d tokens: %w", ordinal, countErr)
		}
		if tokenCount > policy.MaxTokens {
			return nil, fmt.Errorf("chunk %d has %d tokens, exceeds max_tokens %d", ordinal, tokenCount, policy.MaxTokens)
		}
		contentHash := ContentHash([]byte(text))
		duplicateKey := strings.Join([]string{unit.Kind, unit.HeadingPath, contentHash}, "\x00")
		duplicateOrdinal := duplicates[duplicateKey]
		duplicates[duplicateKey] = duplicateOrdinal + 1
		chunkID, idErr := StableChunkID(ChunkIdentityInput{
			KnowledgeScope: document.KnowledgeScope, DocumentID: document.DocumentID,
			StructureKind: unit.Kind, HeadingPath: unit.HeadingPath, Content: text,
			DuplicateOrdinal: duplicateOrdinal,
		})
		if idErr != nil {
			return nil, fmt.Errorf("identify chunk %d: %w", ordinal, idErr)
		}
		chunks = append(chunks, Chunk{
			ChunkID: chunkID, StructureKind: unit.Kind, HeadingPath: unit.HeadingPath,
			Text: text, ContentHash: contentHash, Ordinal: ordinal, TokenCount: tokenCount,
		})
	}
	return chunks, nil
}

func validateChunkPolicy(policy ChunkPolicy) error {
	if policy.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive: %d", policy.MaxTokens)
	}
	if policy.OverlapLines < 0 {
		return fmt.Errorf("overlap_lines must not be negative: %d", policy.OverlapLines)
	}
	// This conservative guard prevents an overlap setting from consuming the
	// entire smallest possible chunk and making line splitting unable to advance.
	if policy.OverlapLines > 0 && policy.OverlapLines >= policy.MaxTokens {
		return fmt.Errorf("overlap_lines must be smaller than max_tokens")
	}
	return nil
}

func splitStructuralUnit(unit structuralUnit, policy ChunkPolicy, counter TokenCounter) ([]structuralUnit, error) {
	unit.Text = normalizePortableText(unit.Text)
	count, err := countTokens(counter, unit.Text)
	if err != nil {
		return nil, err
	}
	if count <= policy.MaxTokens {
		return []structuralUnit{unit}, nil
	}

	switch unit.Kind {
	case "paragraph":
		return splitParagraph(unit, policy.MaxTokens, counter)
	case "code":
		return splitFencedCode(unit, policy, counter)
	case "go_preamble", "go_declaration":
		return splitUnitByLines(unit, policy, counter)
	default:
		return nil, fmt.Errorf("cannot split unsupported structure kind %q", unit.Kind)
	}
}

func packCompatibleUnits(units []structuralUnit, maxTokens int, counter TokenCounter) ([]structuralUnit, error) {
	packed := make([]structuralUnit, 0, len(units))
	for _, unit := range units {
		if len(packed) == 0 || unit.Kind != "paragraph" {
			packed = append(packed, unit)
			continue
		}
		last := &packed[len(packed)-1]
		if last.Kind != unit.Kind || last.HeadingPath != unit.HeadingPath {
			packed = append(packed, unit)
			continue
		}
		candidate := last.Text + "\n\n" + unit.Text
		count, err := countTokens(counter, candidate)
		if err != nil {
			return nil, err
		}
		if count > maxTokens {
			packed = append(packed, unit)
			continue
		}
		last.Text = candidate
	}
	return packed, nil
}

func splitParagraph(unit structuralUnit, maxTokens int, counter TokenCounter) ([]structuralUnit, error) {
	sentences := splitSentences(unit.Text)
	var result []structuralUnit
	current := ""
	flush := func() {
		if current != "" {
			part := unit
			part.Text = current
			result = append(result, part)
			current = ""
		}
	}
	for _, sentence := range sentences {
		pieces, err := splitTextExact(sentence, maxTokens, counter)
		if err != nil {
			return nil, err
		}
		for _, piece := range pieces {
			candidate := piece
			if current != "" {
				candidate = current + piece
			}
			count, countErr := countTokens(counter, candidate)
			if countErr != nil {
				return nil, countErr
			}
			if current != "" && count > maxTokens {
				flush()
				current = piece
			} else {
				current = candidate
			}
		}
	}
	flush()
	return result, nil
}

func splitSentences(text string) []string {
	var result []string
	var current strings.Builder
	for _, char := range text {
		current.WriteRune(char)
		switch char {
		case '.', '!', '?', '。', '！', '？', '\n':
			result = append(result, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func splitTextExact(text string, maxTokens int, counter TokenCounter) ([]string, error) {
	if text == "" {
		return nil, nil
	}
	runes := []rune(text)
	var result []string
	for len(runes) > 0 {
		best := 0
		// BPE merges can make a longer prefix use fewer tokens than a shorter
		// prefix. Evaluate every candidate instead of assuming monotonic counts.
		for end := 1; end <= len(runes); end++ {
			count, err := countTokens(counter, string(runes[:end]))
			if err != nil {
				return nil, err
			}
			if count <= maxTokens {
				best = end
			}
		}
		if best == 0 {
			count, err := countTokens(counter, string(runes[:1]))
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("one rune needs %d tokens, exceeds max_tokens %d", count, maxTokens)
		}
		result = append(result, string(runes[:best]))
		runes = runes[best:]
	}
	return result, nil
}

func splitUnitByLines(unit structuralUnit, policy ChunkPolicy, counter TokenCounter) ([]structuralUnit, error) {
	parts, err := splitLines(strings.Split(unit.Text, "\n"), "", "", policy, counter)
	if err != nil {
		return nil, err
	}
	result := make([]structuralUnit, 0, len(parts))
	for _, text := range parts {
		part := unit
		part.Text = text
		result = append(result, part)
	}
	return result, nil
}

func splitFencedCode(unit structuralUnit, policy ChunkPolicy, counter TokenCounter) ([]structuralUnit, error) {
	lines := strings.Split(unit.Text, "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("fenced code block is malformed")
	}
	parts, err := splitLines(lines[1:len(lines)-1], lines[0], lines[len(lines)-1], policy, counter)
	if err != nil {
		return nil, err
	}
	result := make([]structuralUnit, 0, len(parts))
	for _, text := range parts {
		part := unit
		part.Text = text
		result = append(result, part)
	}
	return result, nil
}

func splitLines(lines []string, prefix, suffix string, policy ChunkPolicy, counter TokenCounter) ([]string, error) {
	wrap := func(body []string) string {
		parts := make([]string, 0, len(body)+2)
		if prefix != "" {
			parts = append(parts, prefix)
		}
		parts = append(parts, body...)
		if suffix != "" {
			parts = append(parts, suffix)
		}
		return strings.Join(parts, "\n")
	}

	var result []string
	coveredEnd := 0
	for start := 0; start < len(lines); {
		bestEnd := start
		for candidateEnd := start + 1; candidateEnd <= len(lines); candidateEnd++ {
			candidate := wrap(lines[start:candidateEnd])
			count, err := countTokens(counter, candidate)
			if err != nil {
				return nil, err
			}
			if count <= policy.MaxTokens {
				bestEnd = candidateEnd
			}
		}
		if start < coveredEnd && bestEnd <= coveredEnd {
			// The overlap fits only by itself and adds no unseen source line.
			// Drop that overlap rather than emitting a duplicate-only chunk.
			start = coveredEnd
			continue
		}
		if bestEnd == start {
			pieces, err := splitWrappedTextExact(lines[start], policy.MaxTokens, counter, wrap)
			if err != nil {
				return nil, err
			}
			result = append(result, pieces...)
			start++
			coveredEnd = start
			continue
		}
		result = append(result, wrap(lines[start:bestEnd]))
		coveredEnd = bestEnd
		if bestEnd == len(lines) {
			break
		}
		next := bestEnd - policy.OverlapLines
		if next <= start {
			next = bestEnd
		}
		start = next
	}
	return result, nil
}

func splitWrappedTextExact(text string, maxTokens int, counter TokenCounter, wrap func([]string) string) ([]string, error) {
	runes := []rune(text)
	var result []string
	for len(runes) > 0 {
		best := 0
		for end := 1; end <= len(runes); end++ {
			candidate := wrap([]string{string(runes[:end])})
			count, err := countTokens(counter, candidate)
			if err != nil {
				return nil, err
			}
			if count <= maxTokens {
				best = end
			}
		}
		if best == 0 {
			candidate := wrap([]string{string(runes[:1])})
			count, err := countTokens(counter, candidate)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("one wrapped rune needs %d tokens, exceeds max_tokens %d", count, maxTokens)
		}
		result = append(result, wrap([]string{string(runes[:best])}))
		runes = runes[best:]
	}
	return result, nil
}

func countTokens(counter TokenCounter, text string) (int, error) {
	count, err := counter.Count(text)
	if err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, fmt.Errorf("token counter returned negative count %d", count)
	}
	if text != "" && count == 0 {
		return 0, fmt.Errorf("token counter returned zero for non-empty text")
	}
	return count, nil
}
