package documentingest

import (
	"fmt"
	"strings"
)

func parseMarkdown(content string) ([]structuralUnit, error) {
	lines := strings.Split(content, "\n")
	headings := make([]string, 6)
	currentPath := "Document"
	var units []structuralUnit
	var paragraph []string
	var code []string
	inFence := false
	fenceChar := byte(0)
	fenceLength := 0

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := normalizePortableText(strings.Join(paragraph, "\n"))
		if text != "" {
			units = append(units, structuralUnit{Kind: "paragraph", HeadingPath: currentPath, Text: text})
		}
		paragraph = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inFence {
			code = append(code, line)
			if isFenceClose(trimmed, fenceChar, fenceLength) {
				units = append(units, structuralUnit{
					Kind: "code", HeadingPath: currentPath, Text: strings.Join(code, "\n"),
				})
				code = nil
				inFence = false
			}
			continue
		}

		if level, title, ok := markdownHeading(line); ok {
			flushParagraph()
			headings[level-1] = title
			for index := level; index < len(headings); index++ {
				headings[index] = ""
			}
			currentPath = joinedHeadingPath(headings)
			continue
		}
		if char, length, ok := fenceOpen(trimmed); ok {
			flushParagraph()
			inFence = true
			fenceChar = char
			fenceLength = length
			code = []string{trimmed}
			continue
		}
		if trimmed == "" {
			flushParagraph()
			continue
		}
		paragraph = append(paragraph, line)
	}
	if inFence {
		return nil, fmt.Errorf("unclosed Markdown fenced code block")
	}
	flushParagraph()
	return units, nil
}

func markdownHeading(line string) (level int, title string, ok bool) {
	// ATX headings may have up to three leading spaces, but must start with one
	// to six # characters followed by whitespace. An arbitrary # in prose is text.
	leading := len(line) - len(strings.TrimLeft(line, " "))
	if leading > 3 {
		return 0, "", false
	}
	line = line[leading:]
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' && line[level] != '\t' {
		return 0, "", false
	}
	title = strings.TrimSpace(line[level:])
	title = strings.TrimSpace(strings.TrimRight(title, "#"))
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func joinedHeadingPath(headings []string) string {
	parts := make([]string, 0, len(headings))
	for _, heading := range headings {
		if heading != "" {
			parts = append(parts, heading)
		}
	}
	if len(parts) == 0 {
		return "Document"
	}
	return strings.Join(parts, " / ")
}

func fenceOpen(line string) (char byte, length int, ok bool) {
	if len(line) < 3 || line[0] != '`' && line[0] != '~' {
		return 0, 0, false
	}
	char = line[0]
	for length < len(line) && line[length] == char {
		length++
	}
	if length < 3 {
		return 0, 0, false
	}
	return char, length, true
}

func isFenceClose(line string, char byte, minimum int) bool {
	if len(line) < minimum {
		return false
	}
	for index := range line {
		if line[index] != char {
			return false
		}
	}
	return true
}
