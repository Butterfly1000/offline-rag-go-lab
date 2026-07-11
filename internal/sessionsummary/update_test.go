package sessionsummary

import (
	"errors"
	"strings"
	"testing"
)

type fakeUpdateStore struct {
	current         SessionSummary
	exists          bool
	getErr          error
	saveErr         error
	saved           SessionSummary
	expectedVersion int64
}

func (s *fakeUpdateStore) Get(_, _ string) (SessionSummary, bool, error) {
	return s.current, s.exists, s.getErr
}

func (s *fakeUpdateStore) Save(next SessionSummary, expectedVersion int64) (SessionSummary, error) {
	s.saved = next
	s.expectedVersion = expectedVersion
	if s.saveErr != nil {
		return SessionSummary{}, s.saveErr
	}
	next.Version = expectedVersion + 1
	return next, nil
}

type fakeMessageSource struct {
	messages      []SourceMessage
	err           error
	lastMessageID int64
	sessionID     string
	userID        string
}

func (s *fakeMessageSource) ListAfter(sessionID, userID string, lastMessageID int64) ([]SourceMessage, error) {
	s.sessionID = sessionID
	s.userID = userID
	s.lastMessageID = lastMessageID
	return s.messages, s.err
}

type fakeSummaryGenerator struct {
	result    string
	err       error
	model     string
	previous  string
	messages  []SourceMessage
	maxTokens int
}

func (g *fakeSummaryGenerator) Update(model, previous string, messages []SourceMessage, maxTokens int) (string, error) {
	g.model = model
	g.previous = previous
	g.messages = messages
	g.maxTokens = maxTokens
	return g.result, g.err
}

func TestUpdateServiceDoesNotGenerateWithoutTrigger(t *testing.T) {
	store := &fakeUpdateStore{
		current: SessionSummary{SessionID: "s", UserID: "u", Content: "old", LastMessageID: 20, Version: 2},
		exists:  true,
	}
	source := &fakeMessageSource{messages: sourceMessages(21, 22)}
	generator := &fakeSummaryGenerator{result: "must not be used"}
	service := mustUpdateService(t, store, source, generator, 8, 2048)

	got, err := service.Update(UpdateRequest{
		SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: 19, MaxOutputTokens: 256,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.Updated || got.Decision.Reason != ReasonNoEvictedMessages || generator.model != "" || store.saved.SessionID != "" {
		t.Fatalf("Update() = %+v generator=%+v saved=%+v", got, generator, store.saved)
	}
	if source.lastMessageID != 20 {
		t.Fatalf("ListAfter() watermark=%d, want 20", source.lastMessageID)
	}
}

func TestUpdateServiceCreatesFirstSummary(t *testing.T) {
	store := &fakeUpdateStore{}
	source := &fakeMessageSource{messages: sourceMessages(1, 2, 3, 4, 5, 6)}
	generator := &fakeSummaryGenerator{result: "first summary"}
	service := mustUpdateService(t, store, source, generator, 4, 100000)

	got, err := service.Update(UpdateRequest{
		SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: 5, MaxOutputTokens: 256,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !got.Updated || got.Summary.Content != "first summary" || got.Summary.LastMessageID != 4 {
		t.Fatalf("Update() = %+v", got)
	}
	assertMessageIDs(t, generator.messages, 1, 2, 3, 4)
	if generator.previous != "" || store.expectedVersion != 0 || store.saved.LastMessageID != 4 {
		t.Fatalf("generator=%+v store=%+v", generator, store)
	}
}

func TestUpdateServiceRollsExistingSummary(t *testing.T) {
	store := &fakeUpdateStore{
		current: SessionSummary{SessionID: "s", UserID: "u", Content: "old summary", LastMessageID: 2, Version: 2},
		exists:  true,
	}
	source := &fakeMessageSource{messages: sourceMessages(3, 4, 5, 6, 7, 8)}
	generator := &fakeSummaryGenerator{result: "rolled summary"}
	service := mustUpdateService(t, store, source, generator, 4, 100000)

	got, err := service.Update(UpdateRequest{
		SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: 7, MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !got.Updated || got.Summary.Version != 3 || got.Summary.LastMessageID != 6 {
		t.Fatalf("Update() = %+v", got)
	}
	if generator.previous != "old summary" || generator.maxTokens != 128 || store.expectedVersion != 2 {
		t.Fatalf("generator=%+v expectedVersion=%d", generator, store.expectedVersion)
	}
}

func TestUpdateServiceDoesNotSaveAfterGeneratorFailure(t *testing.T) {
	store := &fakeUpdateStore{}
	source := &fakeMessageSource{messages: sourceMessages(1, 2, 3)}
	generator := &fakeSummaryGenerator{err: errors.New("ollama down")}
	service := mustUpdateService(t, store, source, generator, 2, 100000)

	_, err := service.Update(UpdateRequest{
		SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: 3, MaxOutputTokens: 128,
	})
	if err == nil || !strings.Contains(err.Error(), "generate") || store.saved.SessionID != "" {
		t.Fatalf("Update() error=%v saved=%+v, want no save", err, store.saved)
	}
}

func TestUpdateServicePropagatesSaveConflict(t *testing.T) {
	store := &fakeUpdateStore{saveErr: ErrVersionConflict}
	service := mustUpdateService(t, store, &fakeMessageSource{messages: sourceMessages(1, 2, 3)}, &fakeSummaryGenerator{result: "summary"}, 2, 100000)

	_, err := service.Update(UpdateRequest{
		SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: 3, MaxOutputTokens: 128,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Update() error=%v, want version conflict", err)
	}
}

func TestUpdateServiceValidatesRequestAndDependencies(t *testing.T) {
	policy := mustTriggerPolicy(t, 2, 100)
	valid := UpdateRequest{SessionID: "s", UserID: "u", Model: "qwen:7b", MaxOutputTokens: 128}

	service := NewUpdateService(nil, &fakeMessageSource{}, fakeMessageTokenCounter{}, policy, &fakeSummaryGenerator{})
	if _, err := service.Update(valid); err == nil {
		t.Fatal("Update() error=nil, want store dependency error")
	}
	service = NewUpdateService(&fakeUpdateStore{}, &fakeMessageSource{}, fakeMessageTokenCounter{}, policy, &fakeSummaryGenerator{})
	invalid := []UpdateRequest{
		{UserID: "u", Model: "qwen:7b", MaxOutputTokens: 128},
		{SessionID: "s", Model: "qwen:7b", MaxOutputTokens: 128},
		{SessionID: "s", UserID: "u", MaxOutputTokens: 128},
		{SessionID: "s", UserID: "u", Model: "qwen:7b", MaxOutputTokens: 0},
		{SessionID: "s", UserID: "u", Model: "qwen:7b", RecentStartID: -1, MaxOutputTokens: 128},
	}
	for _, request := range invalid {
		if _, err := service.Update(request); err == nil {
			t.Fatalf("Update(%+v) error=nil, want validation error", request)
		}
	}
}

func mustUpdateService(t *testing.T, store SummaryStore, source MessageSource, generator SummaryGenerator, minMessages, minTokens int) UpdateService {
	t.Helper()
	policy := mustTriggerPolicy(t, minMessages, minTokens)
	return NewUpdateService(store, source, fakeMessageTokenCounter{}, policy, generator)
}
