package recentchat

import (
	"encoding/json"
	"net/http"

	"offline-rag-go-lab/internal/sessionsummary"
)

func NewService(store MessageStore, window RecentWindowBuilder, ollama OllamaClient) Service {
	return Service{
		store:  store,
		window: window,
		ollama: ollama,
	}
}

// NewServiceWithSessionSummary enables the optional production path while the
// older constructors keep count/manual/automatic callers backward compatible.
func NewServiceWithSessionSummary(
	store MessageStore,
	window RecentWindowBuilder,
	tokenWindow TokenBudgetWindowBuilder,
	ollama OllamaClient,
	automaticBudget AutomaticBudgetPlanner,
	summaryUpdater SessionSummaryUpdater,
	summaryReader SessionSummaryReader,
	summaryInputReserve int,
	summaryOutputLimit int,
) Service {
	return Service{
		store: store, window: window, tokenWindow: tokenWindow, ollama: ollama,
		automaticBudget: automaticBudget,
		summaryUpdater:  summaryUpdater, summaryReader: summaryReader,
		summaryInputReserve: summaryInputReserve, summaryOutputLimit: summaryOutputLimit,
	}
}

var _ SessionSummaryUpdater = sessionsummary.UpdateService{}

func NewServiceWithTokenWindow(store MessageStore, window RecentWindowBuilder, tokenWindow TokenBudgetWindowBuilder, ollama OllamaClient) Service {
	return Service{
		store:       store,
		window:      window,
		tokenWindow: tokenWindow,
		ollama:      ollama,
	}
}

func NewServiceWithAutomaticBudget(
	store MessageStore,
	window RecentWindowBuilder,
	tokenWindow TokenBudgetWindowBuilder,
	ollama OllamaClient,
	automaticBudget AutomaticBudgetPlanner,
) Service {
	return Service{
		store:           store,
		window:          window,
		tokenWindow:     tokenWindow,
		ollama:          ollama,
		automaticBudget: automaticBudget,
	}
}

func RegisterHandlers(mux *http.ServeMux, svc Service) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "recent-chat",
		})
	})

	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}

		defer r.Body.Close()

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}

		resp, err := svc.ChatContext(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	raw, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
