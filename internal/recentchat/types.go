package recentchat

import (
	"errors"
	"strings"
	"time"
)

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

type Message struct {
	ID        int64
	SessionID string
	UserID    string
	Role      MessageRole
	Content   string
	CreatedAt time.Time
}

type ChatRequest struct {
	SessionID       string `json:"session_id"`
	UserID          string `json:"user_id"`
	Message         string `json:"message"`
	Model           string `json:"model"`
	RecentLimit     int    `json:"recent_limit"`
	SystemPrompt    string `json:"system_prompt"`
	StoreUserTurn   bool   `json:"store_user_turn"`
	StoreAssistTurn bool   `json:"store_assistant_turn"`
}

type ChatResponse struct {
	Answer       string    `json:"answer"`
	UsedMessages int       `json:"used_messages"`
	SessionID    string    `json:"session_id"`
	Model        string    `json:"model"`
	CreatedAt    time.Time `json:"created_at"`
	RecentWindow []Message `json:"recent_window"`
}

func (r ChatRequest) Validate() error {
	if strings.TrimSpace(r.SessionID) == "" {
		return errors.New("session_id is required")
	}
	if strings.TrimSpace(r.UserID) == "" {
		return errors.New("user_id is required")
	}
	if strings.TrimSpace(r.Message) == "" {
		return errors.New("message is required")
	}
	return nil
}
