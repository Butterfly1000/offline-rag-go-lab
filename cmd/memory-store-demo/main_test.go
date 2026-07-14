package main

import (
	"strings"
	"testing"

	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/sessionsummary"
)

func TestBuildDemoStepsUsesRealMessageIDsAndExplicitForget(t *testing.T) {
	messages := demoSourceMessages()

	steps, err := buildDemoSteps("s-001", "u-001", messages)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 6 || steps[0].Candidate.SourceMessageIDs[0] != 101 {
		t.Fatalf("steps = %#v", steps)
	}
	last := steps[5]
	if last.Candidate.Operation != memoryitem.OperationForget || last.Candidate.Key != "temporary_tool" || last.SourceMessages[0].Content != demoMessageContents[5] {
		t.Fatalf("forget step = %#v", last)
	}
}

func TestBuildDemoStepsRequiresDedicatedSixMessageSession(t *testing.T) {
	if _, err := buildDemoSteps("s", "u", make([]sessionsummary.SourceMessage, 5)); err == nil {
		t.Fatal("buildDemoSteps() error = nil")
	}
}

func TestBuildDemoStepsRejectsUnexpectedMessageContentAndRole(t *testing.T) {
	messages := demoSourceMessages()
	messages[2].Content = "这不是约定的 Rust 来源。"
	if _, err := buildDemoSteps("s", "u", messages); err == nil || !strings.Contains(err.Error(), "message 3 content") {
		t.Fatalf("unexpected content error = %v", err)
	}

	messages = demoSourceMessages()
	messages[4].Role = "assistant"
	if _, err := buildDemoSteps("s", "u", messages); err == nil || !strings.Contains(err.Error(), "message 5 role") {
		t.Fatalf("unexpected role error = %v", err)
	}
}

func TestClassifyDemoStateIsIdempotentOnlyForExpectedTerminalState(t *testing.T) {
	primary := memoryitem.Item{
		ID: 7, UserID: demoUserID, Kind: memoryitem.KindProjectFact,
		Key: "implementation_language", Value: "Go", Status: memoryitem.StatusActive, Version: 3,
	}
	temporary := memoryitem.Item{
		ID: 8, UserID: demoUserID, Kind: memoryitem.KindPreference,
		Key: "temporary_tool", Value: "Vim", Status: memoryitem.StatusForgotten, Version: 2,
	}

	complete, err := classifyDemoState(primary, true, temporary, true, 6)
	if err != nil || !complete {
		t.Fatalf("complete=%t error=%v", complete, err)
	}
	complete, err = classifyDemoState(memoryitem.Item{}, false, memoryitem.Item{}, false, 0)
	if err != nil || complete {
		t.Fatalf("empty complete=%t error=%v", complete, err)
	}
	if _, err := classifyDemoState(primary, true, temporary, true, 5); err == nil {
		t.Fatal("partial state error = nil")
	}
}

func demoSourceMessages() []sessionsummary.SourceMessage {
	messages := make([]sessionsummary.SourceMessage, len(demoMessageContents))
	for index, content := range demoMessageContents {
		messages[index] = sessionsummary.SourceMessage{
			ID: int64(101 + index), Role: "user", Content: content,
		}
	}
	return messages
}
