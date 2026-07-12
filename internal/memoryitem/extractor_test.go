package memoryitem

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeStructuredGenerator struct {
	response  []byte
	err       error
	model     string
	system    string
	prompt    string
	schema    json.RawMessage
	maxTokens int
}

func (f *fakeStructuredGenerator) GenerateJSON(model, system, prompt string, schema json.RawMessage, maxTokens int) ([]byte, error) {
	f.model = model
	f.system = system
	f.prompt = prompt
	f.schema = append(json.RawMessage(nil), schema...)
	f.maxTokens = maxTokens
	return append([]byte(nil), f.response...), f.err
}

func TestExtractorReturnsValidatedNormalizedCandidates(t *testing.T) {
	generator := &fakeStructuredGenerator{response: []byte(`{
        "candidates":[{
            "operation":"upsert",
            "kind":"project_fact",
            "key":"Implementation Language",
            "value":" Go ",
            "confidence":0.95,
            "source_message_ids":[101]
        }]
    }`)}
	extractor := NewExtractor(generator)

	result, err := extractor.Extract(ExtractRequest{
		Model: "qwen:7b", UserID: "u-001", SessionID: "memory-extract",
		Messages: validExtractionMessages(), MaxOutputTokens: 512,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Key != "implementation_language" || result.Candidates[0].Value != "Go" {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
	if generator.model != "qwen:7b" || generator.maxTokens != 512 {
		t.Fatalf("generator call model=%q maxTokens=%d", generator.model, generator.maxTokens)
	}
	if generator.system != ExtractionSystemPrompt || len(generator.schema) == 0 || !strings.Contains(generator.prompt, "id=101") {
		t.Fatalf("generator call missing system/schema/prompt boundary")
	}
	if string(result.RawJSON) != string(generator.response) {
		t.Fatalf("raw JSON = %q, want %q", result.RawJSON, generator.response)
	}
}

func TestExtractorAllowsNoDurableCandidates(t *testing.T) {
	extractor := NewExtractor(&fakeStructuredGenerator{response: []byte(`{"candidates":[]}`)})
	result, err := extractor.Extract(validExtractRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidates = %#v, want empty", result.Candidates)
	}
}

func TestExtractorRejectsMalformedOrUntrustedOutput(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		{name: "empty response", response: " ", want: "structured response is empty"},
		{name: "invalid JSON", response: `{`, want: "decode memory candidates"},
		{name: "trailing JSON", response: `{"candidates":[]} {}`, want: "trailing JSON"},
		{name: "unknown top field", response: `{"candidates":[],"explanation":"ok"}`, want: "unknown field"},
		{name: "unknown candidate field", response: `{"candidates":[{"operation":"upsert","kind":"goal","key":"next_goal","value":"完成记忆系统","confidence":0.9,"source_message_ids":[101],"reason":"user said so"}]}`, want: "unknown field"},
		{name: "missing confidence", response: `{"candidates":[{"operation":"upsert","kind":"goal","key":"next_goal","value":"完成记忆系统","source_message_ids":[101]}]}`, want: "confidence is required"},
		{name: "unknown source", response: `{"candidates":[{"operation":"upsert","kind":"goal","key":"next_goal","value":"完成记忆系统","confidence":0.9,"source_message_ids":[999]}]}`, want: "source message 999"},
		{name: "assistant source", response: `{"candidates":[{"operation":"upsert","kind":"preference","key":"language","value":"Rust","confidence":0.8,"source_message_ids":[102]}]}`, want: "must have role user"},
	}

	messages := append(validExtractionMessages(), SourceMessage{
		ID: 102, SessionID: "memory-extract", UserID: "u-001", Role: "assistant", Content: "你喜欢 Rust",
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewExtractor(&fakeStructuredGenerator{response: []byte(tt.response)})
			request := validExtractRequest()
			request.Messages = messages
			_, err := extractor.Extract(request)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestExtractorValidatesRequestAndGeneratorFailure(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ExtractRequest)
		want   string
	}{
		{name: "empty model", change: func(r *ExtractRequest) { r.Model = " " }, want: "model is required"},
		{name: "empty user", change: func(r *ExtractRequest) { r.UserID = " " }, want: "user ID is required"},
		{name: "empty session", change: func(r *ExtractRequest) { r.SessionID = " " }, want: "session ID is required"},
		{name: "empty messages", change: func(r *ExtractRequest) { r.Messages = nil }, want: "messages are required"},
		{name: "invalid max tokens", change: func(r *ExtractRequest) { r.MaxOutputTokens = 0 }, want: "max output tokens must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := validExtractRequest()
			tt.change(&request)
			_, err := NewExtractor(&fakeStructuredGenerator{}).Extract(request)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}

	if _, err := NewExtractor(nil).Extract(validExtractRequest()); err == nil || !strings.Contains(err.Error(), "structured generator is required") {
		t.Fatalf("nil generator error = %v", err)
	}
	wantErr := errors.New("ollama unavailable")
	_, err := NewExtractor(&fakeStructuredGenerator{err: wantErr}).Extract(validExtractRequest())
	if !errors.Is(err, wantErr) {
		t.Fatalf("generator error = %v, want wrapping %v", err, wantErr)
	}
}

func validExtractRequest() ExtractRequest {
	return ExtractRequest{
		Model: "qwen:7b", UserID: "u-001", SessionID: "memory-extract",
		Messages: validExtractionMessages(), MaxOutputTokens: 512,
	}
}
