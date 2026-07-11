package recentchat

import (
	"errors"
	"time"

	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/promptbudget"
)

const defaultTokenBudgetFetchLimit = 50

type AutomaticBudgetPlanner interface {
	Plan(model string, fixed []chatprompt.Message, outputReserve int) (promptbudget.AutomaticPlan, error)
}

type Service struct {
	store           MessageStore
	window          RecentWindowBuilder
	tokenWindow     TokenBudgetWindowBuilder
	ollama          OllamaClient
	automaticBudget AutomaticBudgetPlanner
	nowFunc         func() time.Time
}

func (s Service) Chat(req ChatRequest) (ChatResponse, error) {
	if err := req.Validate(); err != nil {
		return ChatResponse{}, err
	}
	if s.store == nil {
		return ChatResponse{}, errors.New("message store is required")
	}
	if s.window == nil {
		return ChatResponse{}, errors.New("recent window builder is required")
	}
	if s.ollama == nil {
		return ChatResponse{}, errors.New("ollama client is required")
	}
	if s.nowFunc == nil {
		s.nowFunc = time.Now
	}

	budgetMode := BudgetModeCount
	historyBudget := req.RecentTokenBudget
	automaticPlan := promptbudget.AutomaticPlan{}
	if req.RecentTokenBudget > 0 {
		budgetMode = BudgetModeManual
	}
	if req.AutoTokenBudget {
		if s.automaticBudget == nil {
			return ChatResponse{}, errors.New("automatic budget planner is required for auto_token_budget")
		}
		if !s.tokenWindow.strict {
			return ChatResponse{}, errors.New("strict token window is required for auto_token_budget")
		}

		fixed := make([]chatprompt.Message, 0, 2)
		if req.SystemPrompt != "" {
			fixed = append(fixed, chatprompt.Message{Role: string(RoleSystem), Content: req.SystemPrompt})
		}
		fixed = append(fixed, chatprompt.Message{Role: string(RoleUser), Content: req.Message})

		var err error
		automaticPlan, err = s.automaticBudget.Plan(req.Model, fixed, req.OutputTokenReserve)
		if err != nil {
			return ChatResponse{}, err
		}
		budgetMode = BudgetModeAutomatic
		historyBudget = automaticPlan.AvailableHistoryTokens
	}

	fetchLimit := req.RecentLimit
	if fetchLimit <= 0 && budgetMode != BudgetModeCount {
		fetchLimit = defaultTokenBudgetFetchLimit
	}

	recent, err := s.store.ListRecentBySession(req.SessionID, fetchLimit)
	if err != nil {
		return ChatResponse{}, err
	}
	selected := s.window.Build(recent, req.RecentLimit)
	usedRecentTokens := 0
	if budgetMode != BudgetModeCount {
		if s.tokenWindow.counter == nil {
			return ChatResponse{}, errors.New("token window builder is required for token budget mode")
		}
		selected, usedRecentTokens, err = s.tokenWindow.Build(recent, historyBudget)
		if err != nil {
			return ChatResponse{}, err
		}
	}

	ollamaMessages := make([]OllamaMessage, 0, len(selected)+2)
	if req.SystemPrompt != "" {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    RoleSystem,
			Content: req.SystemPrompt,
		})
	}
	for _, msg := range selected {
		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	ollamaMessages = append(ollamaMessages, OllamaMessage{
		Role:    RoleUser,
		Content: req.Message,
	})

	ollamaRequest := OllamaChatRequest{
		Model:    req.Model,
		Messages: ollamaMessages,
		Stream:   false,
	}
	if budgetMode == BudgetModeAutomatic {
		ollamaRequest.Options = &OllamaChatOptions{NumPredict: req.OutputTokenReserve}
	}
	chatResp, err := s.ollama.Chat(ollamaRequest)
	if err != nil {
		return ChatResponse{}, err
	}

	now := s.nowFunc()
	if req.StoreUserTurn {
		if err := s.store.Append(Message{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Role:      RoleUser,
			Content:   req.Message,
			CreatedAt: now,
		}); err != nil {
			return ChatResponse{}, err
		}
	}
	if req.StoreAssistTurn {
		if err := s.store.Append(Message{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Role:      RoleAssistant,
			Content:   chatResp.Content,
			CreatedAt: now,
		}); err != nil {
			return ChatResponse{}, err
		}
	}

	return ChatResponse{
		Answer:                chatResp.Content,
		UsedMessages:          len(selected),
		BudgetMode:            budgetMode,
		ContextLimit:          automaticPlan.ContextLimit,
		FixedInputTokens:      automaticPlan.FixedInputTokens,
		OutputTokenReserve:    automaticPlan.OutputReserve,
		AvailableRecentTokens: automaticPlan.AvailableHistoryTokens,
		UsedRecentTokens:      usedRecentTokens,
		SessionID:             req.SessionID,
		Model:                 req.Model,
		CreatedAt:             now,
		RecentWindow:          selected,
	}, nil
}
