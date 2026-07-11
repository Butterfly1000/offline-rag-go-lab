package sessionsummary

import (
	"fmt"
	"strings"
)

type MessageSource interface {
	ListAfter(sessionID, userID string, lastMessageID int64) ([]SourceMessage, error)
}

type SummaryGenerator interface {
	Update(model, previous string, messages []SourceMessage, maxTokens int) (string, error)
}

type TriggerDecider interface {
	Decide(input TriggerInput) (TriggerDecision, error)
}

type UpdateRequest struct {
	SessionID       string
	UserID          string
	Model           string
	RecentStartID   int64
	MaxOutputTokens int
}

type UpdateResult struct {
	Updated   bool
	Decision  TriggerDecision
	Summary   SessionSummary
	Selection PrefixSelection
}

// UpdateService composes the small behaviors from sections 13-16. It saves
// only after selection, trigger evaluation, and generation have all succeeded.
type UpdateService struct {
	store     SummaryStore
	source    MessageSource
	counter   MessageTokenCounter
	policy    TriggerDecider
	generator SummaryGenerator
}

func NewUpdateService(
	store SummaryStore,
	source MessageSource,
	counter MessageTokenCounter,
	policy TriggerDecider,
	generator SummaryGenerator,
) UpdateService {
	return UpdateService{
		store: store, source: source, counter: counter, policy: policy, generator: generator,
	}
}

func (s UpdateService) Update(req UpdateRequest) (UpdateResult, error) {
	if err := validateUpdateRequest(req); err != nil {
		return UpdateResult{}, err
	}
	if s.store == nil {
		return UpdateResult{}, fmt.Errorf("summary store is required")
	}
	if s.source == nil {
		return UpdateResult{}, fmt.Errorf("message source is required")
	}
	if s.counter == nil {
		return UpdateResult{}, fmt.Errorf("message token counter is required")
	}
	if s.policy == nil {
		return UpdateResult{}, fmt.Errorf("trigger policy is required")
	}
	if s.generator == nil {
		return UpdateResult{}, fmt.Errorf("summary generator is required")
	}

	current, exists, err := s.store.Get(req.SessionID, req.UserID)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("get current summary: %w", err)
	}
	if !exists {
		current = SessionSummary{SessionID: req.SessionID, UserID: req.UserID}
	}

	messages, err := s.source.ListAfter(req.SessionID, req.UserID, current.LastMessageID)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("list messages after watermark %d: %w", current.LastMessageID, err)
	}
	selection, err := SelectPrefix(messages, current.LastMessageID, req.RecentStartID, s.counter)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("select summary prefix: %w", err)
	}
	decision, err := s.policy.Decide(TriggerInput{
		UnsummarizedMessages: len(selection.Unsummarized),
		UnsummarizedTokens:   selection.UnsummarizedTokens,
		EvictedMessages:      len(selection.Evicted),
	})
	if err != nil {
		return UpdateResult{}, fmt.Errorf("decide summary trigger: %w", err)
	}

	result := UpdateResult{Decision: decision, Summary: current, Selection: selection}
	if !decision.ShouldSummarize {
		return result, nil
	}
	generated, err := s.generator.Update(
		req.Model,
		current.Content,
		selection.Evicted,
		req.MaxOutputTokens,
	)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("generate rolling summary: %w", err)
	}

	saved, err := s.store.Save(SessionSummary{
		SessionID:     req.SessionID,
		UserID:        req.UserID,
		Content:       generated,
		LastMessageID: selection.NextWatermark,
	}, current.Version)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("save rolling summary: %w", err)
	}
	result.Updated = true
	result.Summary = saved
	return result, nil
}

func validateUpdateRequest(req UpdateRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return fmt.Errorf("session ID is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user ID is required")
	}
	if strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if req.RecentStartID < 0 {
		return fmt.Errorf("recent start ID must not be negative: %d", req.RecentStartID)
	}
	if req.MaxOutputTokens <= 0 {
		return fmt.Errorf("maximum output tokens must be positive: %d", req.MaxOutputTokens)
	}
	return nil
}

var _ TriggerDecider = TriggerPolicy{}
var _ SummaryGenerator = Generator{}
