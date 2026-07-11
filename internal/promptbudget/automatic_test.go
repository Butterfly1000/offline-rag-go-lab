package promptbudget

import (
	"errors"
	"strings"
	"testing"

	"offline-rag-go-lab/internal/chatprompt"
)

type fakeContextProvider struct {
	context int
	err     error
	model   string
}

func (p *fakeContextProvider) ContextLength(model string) (int, error) {
	p.model = model
	return p.context, p.err
}

type fakeConversationCounter struct {
	usage                  chatprompt.TokenUsage
	err                    error
	messages               []chatprompt.Message
	includeAssistantPrefix bool
}

func (c *fakeConversationCounter) Count(messages []chatprompt.Message, includeAssistantPrefix bool) (chatprompt.TokenUsage, error) {
	c.messages = append([]chatprompt.Message(nil), messages...)
	c.includeAssistantPrefix = includeAssistantPrefix
	return c.usage, c.err
}

func TestAutomaticPlannerUsesModelContextAndCompleteFixedPrompt(t *testing.T) {
	provider := &fakeContextProvider{context: 32768}
	counter := &fakeConversationCounter{usage: chatprompt.TokenUsage{
		Rendered:    "rendered fixed prompt",
		TotalTokens: 88,
	}}
	planner := NewAutomaticPlanner(provider, counter)
	fixed := []chatprompt.Message{
		{Role: "system", Content: "规则"},
		{Role: "user", Content: "当前问题"},
	}

	got, err := planner.Plan("qwen:7b", fixed, 2048)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if provider.model != "qwen:7b" {
		t.Fatalf("ContextLength() model = %q, want qwen:7b", provider.model)
	}
	if !counter.includeAssistantPrefix {
		t.Fatal("Count() includeAssistantPrefix = false, want true")
	}
	if len(counter.messages) != 2 || counter.messages[1].Content != "当前问题" {
		t.Fatalf("Count() messages = %#v, want fixed messages", counter.messages)
	}
	if got.ContextLimit != 32768 || got.FixedInputTokens != 88 || got.OutputReserve != 2048 {
		t.Fatalf("Plan() breakdown = %+v, want context=32768 fixed=88 reserve=2048", got)
	}
	if got.AvailableHistoryTokens != 30632 {
		t.Fatalf("Plan() available history = %d, want 30632", got.AvailableHistoryTokens)
	}
	if got.RenderedFixedPrompt != "rendered fixed prompt" {
		t.Fatalf("Plan() rendered = %q", got.RenderedFixedPrompt)
	}
}

func TestAutomaticPlannerPropagatesContextError(t *testing.T) {
	planner := NewAutomaticPlanner(
		&fakeContextProvider{err: errors.New("show failed")},
		&fakeConversationCounter{},
	)

	_, err := planner.Plan("qwen:7b", nil, 2048)
	if err == nil || !strings.Contains(err.Error(), "read model context length") {
		t.Fatalf("Plan() error = %v, want context provider error", err)
	}
}

func TestAutomaticPlannerPropagatesConversationCounterError(t *testing.T) {
	planner := NewAutomaticPlanner(
		&fakeContextProvider{context: 32768},
		&fakeConversationCounter{err: errors.New("tokenizer failed")},
	)

	_, err := planner.Plan("qwen:7b", nil, 2048)
	if err == nil || !strings.Contains(err.Error(), "count fixed prompt tokens") {
		t.Fatalf("Plan() error = %v, want counter error", err)
	}
}

func TestAutomaticPlannerRejectsFixedPromptAndReserveOverContext(t *testing.T) {
	planner := NewAutomaticPlanner(
		&fakeContextProvider{context: 100},
		&fakeConversationCounter{usage: chatprompt.TokenUsage{TotalTokens: 80}},
	)

	_, err := planner.Plan("qwen:7b", nil, 21)
	if err == nil || !strings.Contains(err.Error(), "exceeds context limit") {
		t.Fatalf("Plan() error = %v, want capacity error", err)
	}
}
