package memoryitem

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type StructuredGenerator interface {
	GenerateJSON(model, system, prompt string, schema json.RawMessage, maxTokens int) ([]byte, error)
}

type ExtractRequest struct {
	Model           string
	UserID          string
	SessionID       string
	Summary         string
	Messages        []SourceMessage
	MaxOutputTokens int
}

type ExtractResult struct {
	RawJSON    []byte
	Candidates []Candidate
}

type Extractor struct {
	generator StructuredGenerator
}

func NewExtractor(generator StructuredGenerator) *Extractor {
	return &Extractor{generator: generator}
}

func (e *Extractor) Extract(req ExtractRequest) (ExtractResult, error) {
	if e == nil || e.generator == nil {
		return ExtractResult{}, fmt.Errorf("structured generator is required")
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		return ExtractResult{}, fmt.Errorf("model is required")
	}
	if req.MaxOutputTokens <= 0 {
		return ExtractResult{}, fmt.Errorf("max output tokens must be positive: %d", req.MaxOutputTokens)
	}

	prompt, err := BuildExtractionPrompt(req.UserID, req.SessionID, req.Summary, req.Messages)
	if err != nil {
		return ExtractResult{}, err
	}
	raw, err := e.generator.GenerateJSON(
		req.Model,
		ExtractionSystemPrompt,
		prompt,
		CandidateJSONSchema(),
		req.MaxOutputTokens,
	)
	if err != nil {
		return ExtractResult{}, fmt.Errorf("generate memory candidates: %w", err)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return ExtractResult{}, fmt.Errorf("structured response is empty")
	}

	candidates, err := decodeCandidates(raw)
	if err != nil {
		return ExtractResult{}, err
	}
	validated := make([]Candidate, 0, len(candidates))
	for index, candidate := range candidates {
		normalized, err := ValidateAndNormalizeCandidate(req.UserID, req.SessionID, candidate, req.Messages)
		if err != nil {
			return ExtractResult{}, fmt.Errorf("validate memory candidate %d: %w", index, err)
		}
		validated = append(validated, normalized)
	}
	return ExtractResult{
		RawJSON:    append([]byte(nil), raw...),
		Candidates: validated,
	}, nil
}

type candidateEnvelope struct {
	Candidates *[]rawCandidate `json:"candidates"`
}

type rawCandidate struct {
	Operation        *Operation `json:"operation"`
	Kind             *Kind      `json:"kind"`
	Key              *string    `json:"key"`
	Value            *string    `json:"value"`
	Confidence       *float64   `json:"confidence"`
	SourceMessageIDs *[]int64   `json:"source_message_ids"`
}

func decodeCandidates(raw []byte) ([]Candidate, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var envelope candidateEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode memory candidates: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode memory candidates: trailing JSON value")
		}
		return nil, fmt.Errorf("decode memory candidates trailing JSON: %w", err)
	}
	if envelope.Candidates == nil {
		return nil, fmt.Errorf("decode memory candidates: candidates is required")
	}

	candidates := make([]Candidate, 0, len(*envelope.Candidates))
	for index, rawCandidate := range *envelope.Candidates {
		candidate, err := requireCandidateFields(index, rawCandidate)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func requireCandidateFields(index int, raw rawCandidate) (Candidate, error) {
	if raw.Operation == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: operation is required", index)
	}
	if raw.Kind == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: kind is required", index)
	}
	if raw.Key == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: key is required", index)
	}
	if raw.Value == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: value is required", index)
	}
	if raw.Confidence == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: confidence is required", index)
	}
	if raw.SourceMessageIDs == nil {
		return Candidate{}, fmt.Errorf("decode memory candidate %d: source_message_ids is required", index)
	}
	return Candidate{
		Operation:        *raw.Operation,
		Kind:             *raw.Kind,
		Key:              *raw.Key,
		Value:            *raw.Value,
		Confidence:       *raw.Confidence,
		SourceMessageIDs: append([]int64(nil), (*raw.SourceMessageIDs)...),
	}, nil
}
