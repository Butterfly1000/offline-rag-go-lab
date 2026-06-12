package chunking

import (
	"errors"
	"fmt"
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
)

const (
	defaultChunkMaxChars     = 120
	defaultChunkOverlapChars = 20
)

func BuildChunks(req world.IngestRequest) ([]world.KnowledgeChunk, error) {
	if strings.TrimSpace(req.DocumentID) == "" {
		return nil, errors.New("document_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("text is required")
	}

	lines := strings.Split(req.Text, "\n")
	chunks := make([]world.KnowledgeChunk, 0, len(lines))
	index := 0
	currentSectionTitle := ""
	pendingHeading := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if pendingHeading != "" {
				currentSectionTitle = pendingHeading
				pendingHeading = ""
			}
			continue
		}

		if heading, ok := parseHeadingLine(line); ok {
			pendingHeading = heading
			continue
		}
		if pendingHeading != "" {
			currentSectionTitle = pendingHeading
			pendingHeading = ""
		}

		for _, part := range splitLineWithOverlap(line, defaultChunkMaxChars, defaultChunkOverlapChars) {
			chunks = append(chunks, world.KnowledgeChunk{
				DocumentID: req.DocumentID,
				ChunkID:    fmt.Sprintf("%s#%d", req.DocumentID, index),
				ChunkIndex: index,
				Title:      combineChunkTitle(req.Title, currentSectionTitle),
				SourceRef:  req.SourceRef,
				Text:       part,
				Tags:       append([]string{}, req.Tags...),
			})
			index++
		}
	}

	if len(chunks) == 0 {
		chunks = append(chunks, world.KnowledgeChunk{
			DocumentID: req.DocumentID,
			ChunkID:    fmt.Sprintf("%s#0", req.DocumentID),
			ChunkIndex: 0,
			Title:      req.Title,
			SourceRef:  req.SourceRef,
			Text:       strings.TrimSpace(req.Text),
			Tags:       append([]string{}, req.Tags...),
		})
	}

	return chunks, nil
}

func parseHeadingLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "#") {
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		return heading, heading != ""
	}
	if looksLikeSectionHeading(trimmed) {
		return strings.TrimRight(trimmed, "：:"), true
	}
	return "", false
}

func looksLikeSectionHeading(line string) bool {
	if strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":") {
		return true
	}
	if runeCount(line) > 18 {
		return false
	}
	return strings.HasPrefix(line, "一、") ||
		strings.HasPrefix(line, "二、") ||
		strings.HasPrefix(line, "三、") ||
		strings.HasPrefix(line, "四、") ||
		strings.HasPrefix(line, "五、") ||
		strings.HasPrefix(line, "1.") ||
		strings.HasPrefix(line, "2.") ||
		strings.HasPrefix(line, "3.")
}

func combineChunkTitle(baseTitle, sectionTitle string) string {
	baseTitle = strings.TrimSpace(baseTitle)
	sectionTitle = strings.TrimSpace(sectionTitle)
	if baseTitle == "" {
		return sectionTitle
	}
	if sectionTitle == "" {
		return baseTitle
	}
	return baseTitle + " / " + sectionTitle
}

func splitLineWithOverlap(line string, maxChars int, overlapChars int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if maxChars <= 0 || runeCount(line) <= maxChars {
		return []string{line}
	}

	runes := []rune(line)
	out := make([]string, 0, len(runes)/maxChars+1)
	start := 0
	for start < len(runes) {
		end := start + maxChars
		if end >= len(runes) {
			out = append(out, strings.TrimSpace(string(runes[start:])))
			break
		}
		out = append(out, strings.TrimSpace(string(runes[start:end])))
		start = end - overlapChars
		if start < 0 {
			start = 0
		}
	}
	return out
}

func runeCount(value string) int {
	return len([]rune(value))
}
