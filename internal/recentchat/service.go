package recentchat

import (
	"errors"
	"time"
)

type Service struct {
	store   MessageStore
	window  RecentWindowBuilder
	ollama  OllamaClient
	nowFunc func() time.Time
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

	recent, err := s.store.ListRecentBySession(req.SessionID, req.RecentLimit)
	if err != nil {
		return ChatResponse{}, err
	}
	selected := s.window.Build(recent, req.RecentLimit)

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

	chatResp, err := s.ollama.Chat(OllamaChatRequest{
		Model:    req.Model,
		Messages: ollamaMessages,
		Stream:   false,
	})
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
		Answer:       chatResp.Content,
		UsedMessages: len(selected),
		SessionID:    req.SessionID,
		Model:        req.Model,
		CreatedAt:    now,
		RecentWindow: selected,
	}, nil
}
