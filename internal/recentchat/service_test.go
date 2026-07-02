package recentchat

import (
	"testing"
	"time"
)

func TestChatRequestRequiresSessionUserAndMessage(t *testing.T) {
	req := ChatRequest{}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for empty request")
	}
}

var fixedNow = func() time.Time {
	return time.Date(2026, time.June, 29, 10, 30, 0, 0, time.UTC)
}

func TestServiceChatsWithRecentWindowAndPersistsTurns(t *testing.T) {
	var gotSessionID string
	var gotLimit int
	var appended []Message

	store := &fakeMessageStore{
		listRecentBySessionFn: func(sessionID string, limit int) ([]Message, error) {
			gotSessionID = sessionID
			gotLimit = limit
			return []Message{
				{SessionID: sessionID, UserID: "u1", Role: RoleUser, Content: "old q"},
				{SessionID: sessionID, UserID: "u1", Role: RoleAssistant, Content: "old a"},
			}, nil
		},
		appendFn: func(msg Message) error {
			appended = append(appended, msg)
			return nil
		},
	}
	client := &fakeOllamaClient{
		chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
			if req.Model != "llama3" {
				t.Fatalf("unexpected model: %s", req.Model)
			}
			if req.Stream {
				t.Fatal("expected non-streaming request")
			}
			if len(req.Messages) != 3 {
				t.Fatalf("expected 3 messages, got %d", len(req.Messages))
			}
			if req.Messages[0].Role != RoleUser || req.Messages[0].Content != "old q" {
				t.Fatalf("unexpected first message: %#v", req.Messages[0])
			}
			if req.Messages[1].Role != RoleAssistant || req.Messages[1].Content != "old a" {
				t.Fatalf("unexpected second message: %#v", req.Messages[1])
			}
			if req.Messages[2].Role != RoleUser || req.Messages[2].Content != "new q" {
				t.Fatalf("unexpected final message: %#v", req.Messages[2])
			}
			return OllamaChatResponse{Content: "new answer"}, nil
		},
	}

	svc := Service{
		store:   store,
		window:  CountWindowBuilder{},
		ollama:  client,
		nowFunc: fixedNow,
	}

	resp, err := svc.Chat(ChatRequest{
		SessionID:       "s1",
		UserID:          "u1",
		Message:         "new q",
		Model:           "llama3",
		RecentLimit:     2,
		StoreUserTurn:   true,
		StoreAssistTurn: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotSessionID != "s1" || gotLimit != 2 {
		t.Fatalf("unexpected recent lookup: session=%q limit=%d", gotSessionID, gotLimit)
	}
	if resp.Answer != "new answer" {
		t.Fatalf("unexpected answer: %s", resp.Answer)
	}
	if resp.UsedMessages != 2 {
		t.Fatalf("unexpected used messages: %d", resp.UsedMessages)
	}
	if resp.SessionID != "s1" {
		t.Fatalf("unexpected session: %s", resp.SessionID)
	}
	if resp.Model != "llama3" {
		t.Fatalf("unexpected model: %s", resp.Model)
	}
	if !resp.CreatedAt.Equal(fixedNow()) {
		t.Fatalf("unexpected created at: %s", resp.CreatedAt)
	}
	if len(resp.RecentWindow) != 2 {
		t.Fatalf("unexpected recent window: %#v", resp.RecentWindow)
	}
	if len(appended) != 2 {
		t.Fatalf("expected 2 persisted turns, got %d", len(appended))
	}
	if appended[0].Role != RoleUser || appended[0].Content != "new q" {
		t.Fatalf("unexpected persisted user turn: %#v", appended[0])
	}
	if appended[1].Role != RoleAssistant || appended[1].Content != "new answer" {
		t.Fatalf("unexpected persisted assistant turn: %#v", appended[1])
	}
}

type fakeMessageStore struct {
	listRecentBySessionFn func(sessionID string, limit int) ([]Message, error)
	appendFn              func(msg Message) error
}

func (f *fakeMessageStore) ListRecentBySession(sessionID string, limit int) ([]Message, error) {
	if f.listRecentBySessionFn == nil {
		return nil, nil
	}
	return f.listRecentBySessionFn(sessionID, limit)
}

func (f *fakeMessageStore) Append(msg Message) error {
	if f.appendFn == nil {
		return nil
	}
	return f.appendFn(msg)
}

type fakeOllamaClient struct {
	chatFn func(req OllamaChatRequest) (OllamaChatResponse, error)
}

func (f *fakeOllamaClient) Chat(req OllamaChatRequest) (OllamaChatResponse, error) {
	if f.chatFn == nil {
		return OllamaChatResponse{}, nil
	}
	return f.chatFn(req)
}
