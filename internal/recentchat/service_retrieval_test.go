package recentchat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/contextretrieval"
	"offline-rag-go-lab/internal/promptbudget"
	"offline-rag-go-lab/internal/sessionsummary"
)

func TestChatRequestRetrievalValidationAndOldRequestCompatibility(t *testing.T) {
	old := ChatRequest{SessionID: "s", UserID: "u", Message: "q"}
	if err := old.Validate(); err != nil {
		t.Fatalf("old request validation = %v", err)
	}
	valid := ChatRequest{
		SessionID: "s", UserID: "u", Message: "q",
		AutoTokenBudget: true, OutputTokenReserve: 100,
		UseMemory: true, UseKnowledge: true, KnowledgeScope: "course",
		MemoryLimit: 3, DocumentLimit: 4, ContextTokenBudget: 200,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid retrieval request = %v", err)
	}
	tests := []struct {
		name string
		edit func(*ChatRequest)
		want string
	}{
		{name: "automatic", edit: func(r *ChatRequest) { r.AutoTokenBudget = false }, want: "auto_token_budget"},
		{name: "memory limit", edit: func(r *ChatRequest) { r.MemoryLimit = 0 }, want: "memory_limit"},
		{name: "scope", edit: func(r *ChatRequest) { r.KnowledgeScope = "" }, want: "knowledge_scope"},
		{name: "document limit", edit: func(r *ChatRequest) { r.DocumentLimit = 0 }, want: "document_limit"},
		{name: "context budget", edit: func(r *ChatRequest) { r.ContextTokenBudget = 0 }, want: "context_token_budget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := valid
			tt.edit(&request)
			if err := request.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

type fakeContextRetriever struct {
	result contextretrieval.DualResult
	err    error
	req    contextretrieval.DualRequest
	calls  int
	fn     func(context.Context, contextretrieval.DualRequest) (contextretrieval.DualResult, error)
}

func (r *fakeContextRetriever) Retrieve(ctx context.Context, req contextretrieval.DualRequest) (contextretrieval.DualResult, error) {
	r.calls++
	r.req = req
	if r.fn != nil {
		return r.fn(ctx, req)
	}
	return r.result, r.err
}

type runeTextCounter struct{}

func (runeTextCounter) CountText(text string) (int, []string, []int, error) {
	return utf8.RuneCountInString(text), nil, nil, nil
}

func TestServiceRetrievalDisabledDoesNotCallRetriever(t *testing.T) {
	retriever := &fakeContextRetriever{}
	service := NewServiceWithContextRetrieval(Service{
		store: &fakeMessageStore{}, window: CountWindowBuilder{}, ollama: &fakeOllamaClient{}, nowFunc: fixedNow,
	}, retriever)
	if _, err := service.Chat(ChatRequest{SessionID: "s", UserID: "u", Message: "q", Model: "qwen:7b"}); err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 0 {
		t.Fatalf("retriever calls = %d", retriever.calls)
	}
}

func TestServiceRetrievalInjectsMergedBudgetedContextAndObservations(t *testing.T) {
	memory := retrievalMemoryHit("u-001")
	document := retrievalDocumentHit("course")
	retriever := &fakeContextRetriever{result: contextretrieval.DualResult{
		MemoryHits: []contextretrieval.Hit{memory}, DocumentHits: []contextretrieval.Hit{document},
	}}
	planner := &fakeAutomaticBudgetPlanner{plan: promptbudget.AutomaticPlan{BudgetPlan: promptbudget.BudgetPlan{
		ContextLimit: 1000, FixedInputTokens: 300, OutputReserve: 100, AvailableHistoryTokens: 600,
	}}}
	ollamaCalls := 0
	client := &fakeOllamaClient{chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
		ollamaCalls++
		if len(req.Messages) != 2 || req.Messages[0].Role != RoleSystem {
			t.Fatalf("Ollama messages = %#v", req.Messages)
		}
		system := req.Messages[0].Content
		if strings.Count(system, "base system") != 1 || strings.Count(system, "<retrieved_context>") != 1 {
			t.Fatalf("system prompt duplicated content:\n%s", system)
		}
		return OllamaChatResponse{Content: "answer"}, nil
	}}
	base := Service{
		store: &fakeMessageStore{}, window: CountWindowBuilder{},
		tokenWindow: NewFormattedTokenBudgetWindowBuilder(runeTextCounter{}, chatprompt.QwenFormatter{}),
		ollama:      client, automaticBudget: planner, nowFunc: fixedNow,
	}
	service := NewServiceWithContextRetrieval(base, retriever)
	request := retrievalChatRequest()
	request.SystemPrompt = "base system"
	response, err := service.ChatContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if retriever.calls != 1 || retriever.req.UserID != "u-001" || retriever.req.KnowledgeScope != "course" || retriever.req.MemoryLimit != 3 || retriever.req.DocumentLimit != 4 {
		t.Fatalf("retriever request = %#v calls=%d", retriever.req, retriever.calls)
	}
	if len(planner.fixed) != 2 || strings.Count(planner.fixed[0].Content, "<retrieved_context>") != 1 {
		t.Fatalf("planner fixed = %#v", planner.fixed)
	}
	if ollamaCalls != 1 || len(response.RetrievedContext) != 2 || response.UsedMemoryItems != 1 || response.UsedDocumentChunks != 1 || response.UsedContextTokens <= 0 || response.UsedContextTokens > request.ContextTokenBudget {
		t.Fatalf("response = %#v ollamaCalls=%d", response, ollamaCalls)
	}
}

func TestServiceRetrievalInfrastructureWarningStillCallsOllama(t *testing.T) {
	retriever := &fakeContextRetriever{result: contextretrieval.DualResult{
		MemoryHits: []contextretrieval.Hit{retrievalMemoryHit("u-001")},
		Warnings:   []string{"document infrastructure: unavailable"},
	}}
	ollamaCalls := 0
	service := newRetrievalTestService(retriever, func(OllamaChatRequest) (OllamaChatResponse, error) {
		ollamaCalls++
		return OllamaChatResponse{Content: "memory answer"}, nil
	})
	response, err := service.ChatContext(context.Background(), retrievalChatRequest())
	if err != nil {
		t.Fatal(err)
	}
	if ollamaCalls != 1 || len(response.RetrievalWarnings) != 1 || response.UsedMemoryItems != 1 {
		t.Fatalf("response=%#v ollamaCalls=%d", response, ollamaCalls)
	}
}

func TestServiceRetrievalFixedInputReducesAvailableRecentTokens(t *testing.T) {
	planner := contentLengthPlanner{contextLimit: 2000}
	base := Service{
		store: &fakeMessageStore{}, window: CountWindowBuilder{},
		tokenWindow: NewFormattedTokenBudgetWindowBuilder(runeTextCounter{}, chatprompt.QwenFormatter{}),
		ollama:      &fakeOllamaClient{}, automaticBudget: planner, nowFunc: fixedNow,
	}
	service := NewServiceWithContextRetrieval(base, &fakeContextRetriever{result: contextretrieval.DualResult{
		MemoryHits: []contextretrieval.Hit{retrievalMemoryHit("u-001")},
	}})
	without, err := service.ChatContext(context.Background(), ChatRequest{
		SessionID: "s", UserID: "u-001", Message: "question", Model: "qwen:7b",
		AutoTokenBudget: true, OutputTokenReserve: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	withRequest := retrievalChatRequest()
	withRequest.UseKnowledge = false
	withRequest.KnowledgeScope = ""
	withRequest.DocumentLimit = 0
	with, err := service.ChatContext(context.Background(), withRequest)
	if err != nil {
		t.Fatal(err)
	}
	if with.FixedInputTokens <= without.FixedInputTokens || with.AvailableRecentTokens >= without.AvailableRecentTokens {
		t.Fatalf("without=%#v with=%#v", without, with)
	}
}

func TestServiceRetrievalIntegrityFailurePreventsOllamaAndWrites(t *testing.T) {
	ollamaCalls := 0
	writes := 0
	base := newRetrievalTestService(&fakeContextRetriever{err: contextretrieval.IntegrityFailure(
		contextretrieval.SourceDocument, errors.New("wrong scope"),
	)}, func(OllamaChatRequest) (OllamaChatResponse, error) {
		ollamaCalls++
		return OllamaChatResponse{}, nil
	})
	base.store = &fakeMessageStore{appendFn: func(Message) error { writes++; return nil }}
	request := retrievalChatRequest()
	request.StoreUserTurn = true
	request.StoreAssistTurn = true
	if _, err := base.ChatContext(context.Background(), request); err == nil || !strings.Contains(err.Error(), "wrong scope") {
		t.Fatalf("ChatContext() error = %v", err)
	}
	if ollamaCalls != 0 || writes != 0 {
		t.Fatalf("ollamaCalls=%d writes=%d", ollamaCalls, writes)
	}
}

func TestServiceRetrievalRejectsWrongOwnerHitBeforePrompt(t *testing.T) {
	tests := []struct {
		name   string
		result contextretrieval.DualResult
		want   string
	}{
		{
			name: "user",
			result: contextretrieval.DualResult{
				MemoryHits: []contextretrieval.Hit{retrievalMemoryHit("u-002")},
			},
			want: "belongs to user",
		},
		{
			name: "scope",
			result: contextretrieval.DualResult{
				DocumentHits: []contextretrieval.Hit{retrievalDocumentHit("another-course")},
			},
			want: "belongs to knowledge_scope",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ollamaCalls := 0
			service := newRetrievalTestService(&fakeContextRetriever{result: tt.result}, func(OllamaChatRequest) (OllamaChatResponse, error) {
				ollamaCalls++
				return OllamaChatResponse{}, nil
			})
			if _, err := service.ChatContext(context.Background(), retrievalChatRequest()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ChatContext() error = %v, want %q", err, tt.want)
			}
			if ollamaCalls != 0 {
				t.Fatalf("Ollama calls = %d", ollamaCalls)
			}
		})
	}
}

func TestServiceRetrievalPassesRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	retriever := &fakeContextRetriever{fn: func(got context.Context, _ contextretrieval.DualRequest) (contextretrieval.DualResult, error) {
		return contextretrieval.DualResult{}, got.Err()
	}}
	service := newRetrievalTestService(retriever, nil)
	_, err := service.ChatContext(ctx, retrievalChatRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ChatContext() error = %v", err)
	}
}

func TestServiceRetrievalAndSummaryCombineEachBlockOnce(t *testing.T) {
	formatter := chatprompt.QwenFormatter{}
	planner := &sequenceAutomaticPlanner{plans: []promptbudget.AutomaticPlan{
		{BudgetPlan: promptbudget.BudgetPlan{ContextLimit: 1000, FixedInputTokens: 200, OutputReserve: 100, AvailableHistoryTokens: 700}},
		{BudgetPlan: promptbudget.BudgetPlan{ContextLimit: 1000, FixedInputTokens: 220, OutputReserve: 100, AvailableHistoryTokens: 680}},
	}}
	reader := &fakeSessionSummaryReader{
		summary: sessionsummary.SessionSummary{SessionID: "s", UserID: "u-001", Content: "rolled", LastMessageID: 1, Version: 1},
		exists:  true,
	}
	base := NewServiceWithSessionSummary(
		&fakeMessageStore{}, CountWindowBuilder{},
		NewFormattedTokenBudgetWindowBuilder(fixedOneCounter{}, formatter),
		&fakeOllamaClient{chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
			if len(req.Messages) != 2 {
				t.Fatalf("messages = %#v", req.Messages)
			}
			system := req.Messages[0].Content
			if strings.Count(system, "base") != 1 || strings.Count(system, "<retrieved_context>") != 1 || strings.Count(system, "<session_summary>") != 1 {
				t.Fatalf("combined system = %q", system)
			}
			return OllamaChatResponse{Content: "answer"}, nil
		}},
		planner, &fakeSessionSummaryUpdater{}, reader, 20, 10,
	)
	base.nowFunc = fixedNow
	service := NewServiceWithContextRetrieval(base, &fakeContextRetriever{result: contextretrieval.DualResult{
		MemoryHits: []contextretrieval.Hit{retrievalMemoryHit("u-001")},
	}})
	request := retrievalChatRequest()
	request.SessionID = "s"
	request.SystemPrompt = "base"
	request.UseKnowledge = false
	request.DocumentLimit = 0
	request.KnowledgeScope = ""
	request.UseSessionSummary = true
	response, err := service.ChatContext(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(planner.calls) != 2 || strings.Count(planner.calls[1][0].Content, "<retrieved_context>") != 1 || strings.Count(planner.calls[1][0].Content, "<session_summary>") != 1 {
		t.Fatalf("planner calls = %#v", planner.calls)
	}
	if !response.SessionSummaryUsed || response.AvailableRecentTokens != 680 {
		t.Fatalf("response = %#v", response)
	}
}

type fixedOneCounter struct{}

func (fixedOneCounter) CountText(string) (int, []string, []int, error) { return 1, nil, nil, nil }

type contentLengthPlanner struct{ contextLimit int }

func (p contentLengthPlanner) Plan(_ string, fixed []chatprompt.Message, outputReserve int) (promptbudget.AutomaticPlan, error) {
	used := 0
	for _, message := range fixed {
		used += utf8.RuneCountInString(message.Content)
	}
	return promptbudget.AutomaticPlan{BudgetPlan: promptbudget.BudgetPlan{
		ContextLimit: p.contextLimit, FixedInputTokens: used, OutputReserve: outputReserve,
		AvailableHistoryTokens: p.contextLimit - used - outputReserve,
	}}, nil
}

func newRetrievalTestService(retriever *fakeContextRetriever, chatFn func(OllamaChatRequest) (OllamaChatResponse, error)) Service {
	if chatFn == nil {
		chatFn = func(OllamaChatRequest) (OllamaChatResponse, error) { return OllamaChatResponse{}, nil }
	}
	base := Service{
		store: &fakeMessageStore{}, window: CountWindowBuilder{},
		tokenWindow: NewFormattedTokenBudgetWindowBuilder(runeTextCounter{}, chatprompt.QwenFormatter{}),
		ollama:      &fakeOllamaClient{chatFn: chatFn},
		automaticBudget: &fakeAutomaticBudgetPlanner{plan: promptbudget.AutomaticPlan{BudgetPlan: promptbudget.BudgetPlan{
			ContextLimit: 1000, FixedInputTokens: 200, OutputReserve: 100, AvailableHistoryTokens: 700,
		}}},
		nowFunc: fixedNow,
	}
	return NewServiceWithContextRetrieval(base, retriever)
}

func retrievalChatRequest() ChatRequest {
	return ChatRequest{
		SessionID: "s", UserID: "u-001", Message: "question", Model: "qwen:7b",
		AutoTokenBudget: true, OutputTokenReserve: 100,
		UseMemory: true, UseKnowledge: true, KnowledgeScope: "course",
		MemoryLimit: 3, DocumentLimit: 4, ContextTokenBudget: 500,
	}
}

func retrievalMemoryHit(userID string) contextretrieval.Hit {
	return contextretrieval.Hit{
		Source: contextretrieval.SourceMemory, ID: "memory:1", Content: "project_fact/language: Go",
		Score: 0.9, UserID: userID, Kind: "project_fact",
	}
}

func retrievalDocumentHit(scope string) contextretrieval.Hit {
	return contextretrieval.Hit{
		Source: contextretrieval.SourceDocument, ID: "document:1", Content: "Go course",
		Score: 0.8, KnowledgeScope: scope, Title: "Course", SourceRef: "course.md",
	}
}
