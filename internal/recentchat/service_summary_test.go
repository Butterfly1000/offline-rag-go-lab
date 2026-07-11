package recentchat

import (
	"errors"
	"strings"
	"testing"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/promptbudget"
	"offline-rag-go-lab/internal/sessionsummary"
)

type fakeSessionSummaryUpdater struct {
	result sessionsummary.UpdateResult
	err    error
	req    sessionsummary.UpdateRequest
	events *[]string
}

func (u *fakeSessionSummaryUpdater) Update(req sessionsummary.UpdateRequest) (sessionsummary.UpdateResult, error) {
	u.req = req
	if u.events != nil {
		*u.events = append(*u.events, "update")
	}
	return u.result, u.err
}

type fakeSessionSummaryReader struct {
	summary sessionsummary.SessionSummary
	exists  bool
	err     error
	events  *[]string
}

func (r *fakeSessionSummaryReader) Get(_, _ string) (sessionsummary.SessionSummary, bool, error) {
	if r.events != nil {
		*r.events = append(*r.events, "summary_get")
	}
	return r.summary, r.exists, r.err
}

type sequenceAutomaticPlanner struct {
	plans []promptbudget.AutomaticPlan
	calls [][]chatprompt.Message
}

func (p *sequenceAutomaticPlanner) Plan(_ string, fixed []chatprompt.Message, _ int) (promptbudget.AutomaticPlan, error) {
	p.calls = append(p.calls, append([]chatprompt.Message(nil), fixed...))
	index := len(p.calls) - 1
	if index >= len(p.plans) {
		return promptbudget.AutomaticPlan{}, errors.New("unexpected planner call")
	}
	return p.plans[index], nil
}

func TestServiceUsesUpdatedSummaryWithConservativeRecentWindow(t *testing.T) {
	events := make([]string, 0)
	messages := []Message{
		{ID: 1, Role: RoleUser, Content: "m1"},
		{ID: 2, Role: RoleAssistant, Content: "m2"},
		{ID: 3, Role: RoleUser, Content: "m3"},
	}
	store := &fakeMessageStore{listRecentBySessionFn: func(_ string, _ int) ([]Message, error) {
		events = append(events, "recent_list")
		return messages, nil
	}}
	updater := &fakeSessionSummaryUpdater{
		result: sessionsummary.UpdateResult{
			Updated:  true,
			Decision: sessionsummary.TriggerDecision{ShouldSummarize: true, Reason: sessionsummary.ReasonMessageThreshold},
		},
		events: &events,
	}
	reader := &fakeSessionSummaryReader{
		summary: sessionsummary.SessionSummary{SessionID: "s", UserID: "u", Content: "rolled", LastMessageID: 1, Version: 2},
		exists:  true,
		events:  &events,
	}
	planner := &sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{
		{BudgetPlan: promptbudget.BudgetPlan{ContextLimit: 200, FixedInputTokens: 80, OutputReserve: 20, AvailableHistoryTokens: 100}},
		{BudgetPlan: promptbudget.BudgetPlan{ContextLimit: 200, FixedInputTokens: 95, OutputReserve: 20, AvailableHistoryTokens: 85}},
	}}
	formatter := chatprompt.QwenFormatter{}
	counts := make(map[string]int)
	for _, message := range messages {
		formatted, err := formatter.FormatMessage(chatprompt.Message{Role: string(message.Role), Content: message.Content})
		if err != nil {
			t.Fatal(err)
		}
		counts[formatted] = 40
	}
	summaryFormatted, err := formatter.FormatMessage(chatprompt.Message{Role: "system", Content: sessionSummaryBlock("rolled")})
	if err != nil {
		t.Fatal(err)
	}
	counts[summaryFormatted] = 15

	client := &fakeOllamaClient{chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
		events = append(events, "ollama")
		if len(req.Messages) != 4 || req.Messages[0].Role != RoleSystem {
			t.Fatalf("Ollama messages = %#v, want combined system + m2 + m3 + user", req.Messages)
		}
		if !strings.Contains(req.Messages[0].Content, "base system") || !strings.Contains(req.Messages[0].Content, "rolled") {
			t.Fatalf("combined system = %q", req.Messages[0].Content)
		}
		if req.Messages[1].Content != "m2" || req.Messages[2].Content != "m3" {
			t.Fatalf("selected history = %#v", req.Messages)
		}
		return OllamaChatResponse{Content: "answer"}, nil
	}}
	service := NewServiceWithSessionSummary(
		store,
		CountWindowBuilder{},
		NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{counts: counts}, formatter),
		client,
		planner,
		updater,
		reader,
		20,
		10,
	)
	service.nowFunc = fixedNow

	resp, err := service.Chat(ChatRequest{
		SessionID: "s", UserID: "u", Message: "current", Model: "qwen:7b",
		SystemPrompt: "base system", AutoTokenBudget: true, OutputTokenReserve: 20,
		UseSessionSummary: true,
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if updater.req.RecentStartID != 2 || updater.req.MaxOutputTokens != 10 {
		t.Fatalf("updater request = %+v", updater.req)
	}
	if got := strings.Join(events, ","); got != "recent_list,update,summary_get,ollama" {
		t.Fatalf("operation order = %s", got)
	}
	if len(planner.calls) != 2 || len(planner.calls[1]) != 2 || !strings.Contains(planner.calls[1][0].Content, "rolled") {
		t.Fatalf("planner calls = %#v", planner.calls)
	}
	if resp.UsedMessages != 2 || resp.UsedRecentTokens != 80 || resp.AvailableRecentTokens != 85 {
		t.Fatalf("recent response = %+v", resp)
	}
	if !resp.SessionSummaryUsed || !resp.SessionSummaryUpdated || resp.SessionSummaryVersion != 2 || resp.SessionSummaryWatermark != 1 || resp.SessionSummaryTriggerReason != sessionsummary.ReasonMessageThreshold {
		t.Fatalf("summary response = %+v", resp)
	}
}

func TestServiceRejectsOversizedSummaryAndFinalBudgetRegression(t *testing.T) {
	formatter := chatprompt.QwenFormatter{}
	block := sessionSummaryBlock("large")
	formatted, err := formatter.FormatMessage(chatprompt.Message{Role: "system", Content: block})
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeSessionSummaryReader{
		summary: sessionsummary.SessionSummary{SessionID: "s", UserID: "u", Content: "large", LastMessageID: 1, Version: 1},
		exists:  true,
	}
	request := ChatRequest{
		SessionID: "s", UserID: "u", Message: "current", Model: "qwen:7b",
		AutoTokenBudget: true, OutputTokenReserve: 20, UseSessionSummary: true,
	}

	oversized := NewServiceWithSessionSummary(
		&fakeMessageStore{}, CountWindowBuilder{},
		NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{counts: map[string]int{formatted: 21}}, formatter),
		&fakeOllamaClient{},
		&sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{{BudgetPlan: promptbudget.BudgetPlan{AvailableHistoryTokens: 100}}}},
		&fakeSessionSummaryUpdater{}, reader, 20, 10,
	)
	if _, err := oversized.Chat(request); err == nil || !strings.Contains(err.Error(), "summary input reserve") {
		t.Fatalf("Chat() error = %v, want oversized summary error", err)
	}

	regression := NewServiceWithSessionSummary(
		&fakeMessageStore{}, CountWindowBuilder{},
		NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{counts: map[string]int{formatted: 15}}, formatter),
		&fakeOllamaClient{},
		&sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{
			{BudgetPlan: promptbudget.BudgetPlan{AvailableHistoryTokens: 100}},
			{BudgetPlan: promptbudget.BudgetPlan{AvailableHistoryTokens: 79}},
		}},
		&fakeSessionSummaryUpdater{}, reader, 20, 10,
	)
	if _, err := regression.Chat(request); err == nil || !strings.Contains(err.Error(), "conservative history budget") {
		t.Fatalf("Chat() error = %v, want final budget regression error", err)
	}
}

func TestServiceRequiresValidSessionSummaryDependenciesAndConfig(t *testing.T) {
	request := ChatRequest{
		SessionID: "s", UserID: "u", Message: "current", Model: "qwen:7b",
		AutoTokenBudget: true, OutputTokenReserve: 20, UseSessionSummary: true,
	}
	baseArgs := func(updater SessionSummaryUpdater, reader SessionSummaryReader, inputReserve, outputLimit int) Service {
		return NewServiceWithSessionSummary(
			&fakeMessageStore{}, CountWindowBuilder{},
			NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{}, chatprompt.QwenFormatter{}),
			&fakeOllamaClient{},
			&sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{{BudgetPlan: promptbudget.BudgetPlan{AvailableHistoryTokens: 100}}}},
			updater, reader, inputReserve, outputLimit,
		)
	}
	reader := &fakeSessionSummaryReader{}
	updater := &fakeSessionSummaryUpdater{}
	tests := []Service{
		baseArgs(nil, reader, 20, 10),
		baseArgs(updater, nil, 20, 10),
		baseArgs(updater, reader, 0, 10),
		baseArgs(updater, reader, 20, 20),
	}
	for i, service := range tests {
		if _, err := service.Chat(request); err == nil {
			t.Fatalf("case %d Chat() error=nil, want dependency/config error", i)
		}
	}
}

func TestServicePropagatesSessionSummaryUpdateAndReadErrors(t *testing.T) {
	request := ChatRequest{
		SessionID: "s", UserID: "u", Message: "current", Model: "qwen:7b",
		AutoTokenBudget: true, OutputTokenReserve: 20, UseSessionSummary: true,
	}
	newService := func(updater *fakeSessionSummaryUpdater, reader *fakeSessionSummaryReader) Service {
		return NewServiceWithSessionSummary(
			&fakeMessageStore{}, CountWindowBuilder{},
			NewFormattedTokenBudgetWindowBuilder(fakeTokenCounter{}, chatprompt.QwenFormatter{}),
			&fakeOllamaClient{},
			&sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{{BudgetPlan: promptbudget.BudgetPlan{AvailableHistoryTokens: 100}}}},
			updater, reader, 20, 10,
		)
	}
	if _, err := newService(&fakeSessionSummaryUpdater{err: errors.New("update failed")}, &fakeSessionSummaryReader{}).Chat(request); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("Chat() error = %v, want update error", err)
	}
	if _, err := newService(&fakeSessionSummaryUpdater{}, &fakeSessionSummaryReader{err: errors.New("read failed")}).Chat(request); err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("Chat() error = %v, want read error", err)
	}
}
