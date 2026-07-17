package documentingest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
)

const (
	maxIdentityLength      = 128
	maxStructureKindLength = 64
	maxSourceRefLength     = 1024
	maxHeadingPathLength   = 1024
)

func NormalizeDocument(input Document) (Document, error) {
	var err error
	input.KnowledgeScope, err = normalizeIdentifier("knowledge_scope", input.KnowledgeScope)
	if err != nil {
		return Document{}, err
	}
	input.DocumentID, err = normalizeIdentifier("document_id", input.DocumentID)
	if err != nil {
		return Document{}, err
	}
	input.SourceRef, err = normalizeSourceRef(input.SourceRef)
	if err != nil {
		return Document{}, err
	}
	input.Format, err = normalizeFormat(input.Format)
	if err != nil {
		return Document{}, err
	}
	content := normalizePortableText(string(input.Content))
	if content == "" {
		return Document{}, fmt.Errorf("document content is required")
	}
	// Converting the normalized string back to bytes gives the result ownership
	// of its own backing array instead of retaining the caller's mutable slice.
	input.Content = []byte(content)
	return input, nil
}

func ContentHash(content []byte) string {
	return sha256Hex(normalizePortableText(string(content)))
}

func ChunkPolicyHash(policy ChunkPolicyIdentity) (string, error) {
	format, err := normalizeFormat(policy.Format)
	if err != nil {
		return "", err
	}
	parserVersion, err := normalizeIdentifier("parser_version", policy.ParserVersion)
	if err != nil {
		return "", err
	}
	if policy.MaxTokens <= 0 {
		return "", fmt.Errorf("max_tokens must be positive: %d", policy.MaxTokens)
	}
	if policy.OverlapLines < 0 {
		return "", fmt.Errorf("overlap_lines must not be negative: %d", policy.OverlapLines)
	}
	canonical := strings.Join([]string{
		string(format), parserVersion, strconv.Itoa(policy.MaxTokens), strconv.Itoa(policy.OverlapLines),
	}, "\x00")
	return sha256Hex(canonical), nil
}

func StableChunkID(input ChunkIdentityInput) (string, error) {
	knowledgeScope, err := normalizeIdentifier("knowledge_scope", input.KnowledgeScope)
	if err != nil {
		return "", err
	}
	documentID, err := normalizeIdentifier("document_id", input.DocumentID)
	if err != nil {
		return "", err
	}
	structureKind, err := normalizeIdentifier("structure_kind", input.StructureKind)
	if err != nil {
		return "", err
	}
	if len(structureKind) > maxStructureKindLength {
		return "", fmt.Errorf("structure_kind exceeds %d bytes", maxStructureKindLength)
	}
	headingPath, err := normalizeHeadingPath(input.HeadingPath)
	if err != nil {
		return "", err
	}
	content := normalizePortableText(input.Content)
	if content == "" {
		return "", fmt.Errorf("chunk content is required")
	}
	if input.DuplicateOrdinal < 0 {
		return "", fmt.Errorf("duplicate_ordinal must not be negative: %d", input.DuplicateOrdinal)
	}
	canonical := strings.Join([]string{
		knowledgeScope,
		documentID,
		structureKind,
		headingPath,
		sha256Hex(content),
		strconv.Itoa(input.DuplicateOrdinal),
	}, "\x00")
	return sha256Hex(canonical), nil
}

func normalizeIdentifier(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if len(value) > maxIdentityLength {
		return "", fmt.Errorf("%s exceeds %d bytes", field, maxIdentityLength)
	}
	for index, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.' || char == ':'
		if !valid || index == 0 && (char == '.' || char == ':' || char == '-') {
			return "", fmt.Errorf("%s contains unsafe character %q", field, char)
		}
	}
	return value, nil
}

func normalizeSourceRef(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("source_ref is required")
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("source_ref must be slash-separated")
	}
	if len(value) > maxSourceRefLength {
		return "", fmt.Errorf("source_ref exceeds %d bytes", maxSourceRefLength)
	}
	if containsControl(value) {
		return "", fmt.Errorf("source_ref must not contain control characters")
	}
	if path.IsAbs(value) {
		return "", fmt.Errorf("source_ref must be relative")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("source_ref must not escape the project")
	}
	if cleaned != value {
		return "", fmt.Errorf("source_ref must be clean: use %q", cleaned)
	}
	return value, nil
}

func normalizeFormat(value DocumentFormat) (DocumentFormat, error) {
	value = DocumentFormat(strings.ToLower(strings.TrimSpace(string(value))))
	switch value {
	case FormatMarkdown, FormatGo:
		return value, nil
	case "":
		return "", fmt.Errorf("document format is required")
	default:
		return "", fmt.Errorf("unsupported document format %q", value)
	}
}

func normalizeHeadingPath(value string) (string, error) {
	if len(value) > maxHeadingPathLength {
		return "", fmt.Errorf("heading_path exceeds %d bytes", maxHeadingPathLength)
	}
	if containsControl(value) {
		return "", fmt.Errorf("heading_path must not contain control characters")
	}
	parts := strings.Split(value, "/")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
		if parts[index] == "" {
			return "", fmt.Errorf("heading_path contains an empty segment")
		}
	}
	result := strings.Join(parts, " / ")
	if result == "" {
		return "", fmt.Errorf("heading_path is required")
	}
	return result, nil
}

func normalizePortableText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " \t")
	}
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func containsControl(value string) bool {
	for _, char := range value {
		if char < ' ' || char == 0x7f {
			return true
		}
	}
	return false
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
