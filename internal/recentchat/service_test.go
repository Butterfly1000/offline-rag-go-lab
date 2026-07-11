package recentchat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/promptbudget"
)

func TestChatRequestRequiresSessionUserAndMessage(t *testing.T) {
	req := ChatRequest{}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for empty request")
	}
}

func TestChatRequestRejectsConflictingAutomaticAndManualBudgets(t *testing.T) {
	req := ChatRequest{
		SessionID:          "s1",
		UserID:             "u1",
		Message:            "hello",
		AutoTokenBudget:    true,
		RecentTokenBudget:  100,
		OutputTokenReserve: 2048,
	}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "cannot be used together") {
		t.Fatalf("Validate() error = %v, want conflicting budget error", err)
	}
}

func TestChatRequestRequiresOutputReserveForAutomaticBudget(t *testing.T) {
	req := ChatRequest{
		SessionID:       "s1",
		UserID:          "u1",
		Message:         "hello",
		AutoTokenBudget: true,
	}
	if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "output_token_reserve") {
		t.Fatalf("Validate() error = %v, want output reserve error", err)
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
		store:       store,
		window:      CountWindowBuilder{},
		tokenWindow: NewTokenBudgetWindowBuilder(fakeTokenCounter{counts: map[string]int{"old q": 2, "old a": 2}}),
		ollama:      client,
		nowFunc:     fixedNow,
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
	if resp.BudgetMode != BudgetModeCount {
		t.Fatalf("unexpected budget mode: %q", resp.BudgetMode)
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

func TestServiceUsesTokenBudgetWindowWhenProvided(t *testing.T) {
	store := &fakeMessageStore{
		listRecentBySessionFn: func(sessionID string, limit int) ([]Message, error) {
			return []Message{
				{SessionID: sessionID, UserID: "u1", Role: RoleUser, Content: "old q"},
				{SessionID: sessionID, UserID: "u1", Role: RoleAssistant, Content: "old a"},
			}, nil
		},
	}
	client := &fakeOllamaClient{
		chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
			if len(req.Messages) != 2 {
				t.Fatalf("expected 2 messages, got %d", len(req.Messages))
			}
			if req.Messages[0].Content != "old a" {
				t.Fatalf("expected newest message only, got %#v", req.Messages[0])
			}
			return OllamaChatResponse{Content: "ok"}, nil
		},
	}

	svc := Service{
		store:  store,
		window: CountWindowBuilder{},
		tokenWindow: NewTokenBudgetWindowBuilder(fakeTokenCounter{
			counts: map[string]int{
				"old q": 5,
				"old a": 2,
			},
		}),
		ollama:  client,
		nowFunc: fixedNow,
	}

	resp, err := svc.Chat(ChatRequest{
		SessionID:         "s1",
		UserID:            "u1",
		Message:           "new q",
		Model:             "llama3",
		RecentLimit:       10,
		RecentTokenBudget: 3,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.UsedMessages != 1 {
		t.Fatalf("unexpected used messages: %d", resp.UsedMessages)
	}
	if resp.UsedRecentTokens != 2 {
		t.Fatalf("unexpected used recent tokens: %d", resp.UsedRecentTokens)
	}
	if len(resp.RecentWindow) != 1 || resp.RecentWindow[0].Content != "old a" {
		t.Fatalf("unexpected recent window: %#v", resp.RecentWindow)
	}
	if resp.BudgetMode != BudgetModeManual {
		t.Fatalf("unexpected budget mode: %q", resp.BudgetMode)
	}
}

func TestServiceUsesAutomaticBudgetAndReturnsBreakdown(t *testing.T) {
	var gotLimit int
	store := &fakeMessageStore{
		listRecentBySessionFn: func(_ string, limit int) ([]Message, error) {
			gotLimit = limit
			return []Message{
				{Role: RoleUser, Content: "old q"},
				{Role: RoleAssistant, Content: "old a"},
			}, nil
		},
	}
	planner := &fakeAutomaticBudgetPlanner{plan: promptbudget.AutomaticPlan{
		BudgetPlan: promptbudget.BudgetPlan{
			ContextLimit:           32768,
			FixedInputTokens:       88,
			OutputReserve:          2048,
			AvailableHistoryTokens: 3,
		},
		RenderedFixedPrompt: "fixed",
	}}
	formatter := chatprompt.QwenFormatter{}
	formattedOldQ, err := formatter.FormatMessage(chatprompt.Message{Role: "user", Content: "old q"})
	if err != nil {
		t.Fatalf("format old q: %v", err)
	}
	formattedOldA, err := formatter.FormatMessage(chatprompt.Message{Role: "assistant", Content: "old a"})
	if err != nil {
		t.Fatalf("format old a: %v", err)
	}
	client := &fakeOllamaClient{chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
		if len(req.Messages) != 3 {
			t.Fatalf("Ollama messages = %#v, want system + selected history + current user", req.Messages)
		}
		if req.Messages[1].Role != RoleAssistant || req.Messages[1].Content != "old a" {
			t.Fatalf("selected history = %#v, want newest assistant", req.Messages[1])
		}
		if req.Options == nil || req.Options.NumPredict != 2048 {
			t.Fatalf("Ollama options = %#v, want num_predict 2048", req.Options)
		}
		return OllamaChatResponse{Content: "answer"}, nil
	}}
	svc := Service{
		store:  store,
		window: CountWindowBuilder{},
		tokenWindow: NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{counts: map[string]int{
			formattedOldQ: 5,
			formattedOldA: 2,
		}}, formatter),
		ollama:          client,
		automaticBudget: planner,
		nowFunc:         fixedNow,
	}

	resp, err := svc.Chat(ChatRequest{
		SessionID:          "s1",
		UserID:             "u1",
		Message:            "new q",
		Model:              "qwen:7b",
		SystemPrompt:       "system",
		AutoTokenBudget:    true,
		OutputTokenReserve: 2048,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if gotLimit != 50 {
		t.Fatalf("recent fetch limit = %d, want default 50", gotLimit)
	}
	if planner.model != "qwen:7b" || planner.outputReserve != 2048 {
		t.Fatalf("planner input = model %q reserve %d", planner.model, planner.outputReserve)
	}
	if len(planner.fixed) != 2 || planner.fixed[0].Role != "system" || planner.fixed[1].Role != "user" {
		t.Fatalf("planner fixed messages = %#v", planner.fixed)
	}
	if resp.BudgetMode != BudgetModeAutomatic || resp.UsedMessages != 1 || resp.UsedRecentTokens != 2 {
		t.Fatalf("automatic response selection = %+v", resp)
	}
	if resp.ContextLimit != 32768 || resp.FixedInputTokens != 88 || resp.OutputTokenReserve != 2048 || resp.AvailableRecentTokens != 3 {
		t.Fatalf("automatic response breakdown = %+v", resp)
	}
}

func TestAutomaticBudgetResponseKeepsZeroBreakdownFields(t *testing.T) {
	raw, err := json.Marshal(ChatResponse{BudgetMode: BudgetModeAutomatic})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, field := range []string{
		`"context_limit":0`,
		`"fixed_input_tokens":0`,
		`"output_token_reserve":0`,
		`"available_recent_tokens":0`,
		`"used_recent_tokens":0`,
	} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("Marshal() = %s, want field %s", raw, field)
		}
	}
}

func TestServicePropagatesAutomaticBudgetError(t *testing.T) {
	svc := Service{
		store:           &fakeMessageStore{},
		window:          CountWindowBuilder{},
		tokenWindow:     NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{}, chatprompt.QwenFormatter{}),
		ollama:          &fakeOllamaClient{},
		automaticBudget: &fakeAutomaticBudgetPlanner{err: errors.New("plan failed")},
	}

	_, err := svc.Chat(ChatRequest{
		SessionID:          "s1",
		UserID:             "u1",
		Message:            "hello",
		Model:              "qwen:7b",
		AutoTokenBudget:    true,
		OutputTokenReserve: 2048,
	})
	if err == nil || !strings.Contains(err.Error(), "plan failed") {
		t.Fatalf("Chat() error = %v, want planner error", err)
	}
}

func TestServiceRequiresStrictWindowForAutomaticBudget(t *testing.T) {
	svc := Service{
		store:           &fakeMessageStore{},
		window:          CountWindowBuilder{},
		tokenWindow:     NewTokenBudgetWindowBuilder(fakeTokenCounter{}),
		ollama:          &fakeOllamaClient{},
		automaticBudget: &fakeAutomaticBudgetPlanner{},
	}

	_, err := svc.Chat(ChatRequest{
		SessionID:          "s1",
		UserID:             "u1",
		Message:            "hello",
		Model:              "qwen:7b",
		AutoTokenBudget:    true,
		OutputTokenReserve: 2048,
	})
	if err == nil || !strings.Contains(err.Error(), "strict token window") {
		t.Fatalf("Chat() error = %v, want strict window error", err)
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

type fakeAutomaticBudgetPlanner struct {
	plan          promptbudget.AutomaticPlan
	err           error
	model         string
	fixed         []chatprompt.Message
	outputReserve int
}

func (p *fakeAutomaticBudgetPlanner) Plan(model string, fixed []chatprompt.Message, outputReserve int) (promptbudget.AutomaticPlan, error) {
	p.model = model
	p.fixed = append([]chatprompt.Message(nil), fixed...)
	p.outputReserve = outputReserve
	return p.plan, p.err
}

func (f *fakeOllamaClient) Chat(req OllamaChatRequest) (OllamaChatResponse, error) {
	if f.chatFn == nil {
		return OllamaChatResponse{}, nil
	}
	return f.chatFn(req)
}
