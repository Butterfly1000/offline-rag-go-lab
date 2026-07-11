package recentchat

import (
	"errors"
	"strings"
	"time"

	"offline-rag-go-lab/internal/sessionsummary"
)

type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

type BudgetMode string

const (
	BudgetModeCount     BudgetMode = "count"
	BudgetModeManual    BudgetMode = "manual"
	BudgetModeAutomatic BudgetMode = "automatic"
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
	SessionID          string `json:"session_id"`
	UserID             string `json:"user_id"`
	Message            string `json:"message"`
	Model              string `json:"model"`
	RecentLimit        int    `json:"recent_limit"`
	RecentTokenBudget  int    `json:"recent_token_budget"`
	AutoTokenBudget    bool   `json:"auto_token_budget"`
	OutputTokenReserve int    `json:"output_token_reserve"`
	SystemPrompt       string `json:"system_prompt"`
	StoreUserTurn      bool   `json:"store_user_turn"`
	StoreAssistTurn    bool   `json:"store_assistant_turn"`
	UseSessionSummary  bool   `json:"use_session_summary"`
}

type ChatResponse struct {
	Answer                      string                       `json:"answer"`
	UsedMessages                int                          `json:"used_messages"`
	BudgetMode                  BudgetMode                   `json:"budget_mode"`
	ContextLimit                int                          `json:"context_limit"`
	FixedInputTokens            int                          `json:"fixed_input_tokens"`
	OutputTokenReserve          int                          `json:"output_token_reserve"`
	AvailableRecentTokens       int                          `json:"available_recent_tokens"`
	UsedRecentTokens            int                          `json:"used_recent_tokens"`
	SessionID                   string                       `json:"session_id"`
	Model                       string                       `json:"model"`
	CreatedAt                   time.Time                    `json:"created_at"`
	RecentWindow                []Message                    `json:"recent_window"`
	SessionSummaryUsed          bool                         `json:"session_summary_used"`
	SessionSummaryUpdated       bool                         `json:"session_summary_updated"`
	SessionSummaryVersion       int64                        `json:"session_summary_version"`
	SessionSummaryWatermark     int64                        `json:"session_summary_watermark"`
	SessionSummaryTriggerReason sessionsummary.TriggerReason `json:"session_summary_trigger_reason"`
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
	if r.AutoTokenBudget && r.RecentTokenBudget > 0 {
		return errors.New("auto_token_budget and recent_token_budget cannot be used together")
	}
	if r.AutoTokenBudget && r.OutputTokenReserve <= 0 {
		return errors.New("output_token_reserve must be positive when auto_token_budget is enabled")
	}
	if r.UseSessionSummary && !r.AutoTokenBudget {
		return errors.New("use_session_summary requires auto_token_budget")
	}
	return nil
}
