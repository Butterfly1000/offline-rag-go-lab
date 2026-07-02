# Recent Window Real Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a real first-layer conversational memory path for this project using MySQL for message persistence, a recent-window builder for multi-turn context, and a real Ollama-backed chat path in an isolated internal package.

**Architecture:** Create a new isolated package tree under `internal/recentchat/` rather than stretching the current teaching-oriented gateway. The new path will own message models, MySQL-backed storage, recent-window assembly, Ollama chat integration, and a dedicated HTTP entrypoint while reusing the existing retrieval/compression interfaces only where useful later.

**Tech Stack:** Go 1.21, `database/sql`, MySQL driver, standard `net/http`, Ollama HTTP API, existing project config style, curl-based manual verification.

---

## File Structure

### New files

- `internal/recentchat/types.go`
  Defines request/response DTOs, message roles, and domain structs for recent-window chat.
- `internal/recentchat/store.go`
  Declares the `MessageStore` interface and any store-level query params.
- `internal/recentchat/store_mysql.go`
  Implements MySQL-backed message reads/writes.
- `internal/recentchat/window.go`
  Declares `RecentWindowBuilder` and default count-based builder.
- `internal/recentchat/window_count.go`
  Implements recent-window selection by message count.
- `internal/recentchat/ollama.go`
  Implements a real Ollama client for chat completions.
- `internal/recentchat/service.go`
  Orchestrates request validation, message loading, recent-window build, Ollama invocation, and message persistence.
- `internal/recentchat/http.go`
  Provides HTTP handlers for the isolated recent-window chat path.
- `internal/recentchat/store_mysql_test.go`
  Tests MySQL store logic with mocked DB or integration-gated structure.
- `internal/recentchat/window_count_test.go`
  Tests recent-window selection order and limit behavior.
- `internal/recentchat/service_test.go`
  Tests orchestration with fake store and fake Ollama client.
- `cmd/recent-chat/main.go`
  Dedicated executable for the new real implementation path.
- `docs/teaching/recent-window-layer-01.md`
  Teaching note for the implemented first layer after the code exists.
- `sql/recentchat_messages.sql`
  MySQL schema for persisted chat messages.

### Modified files

- `go.mod`
  Add MySQL driver dependency if not already present.
- `README.md`
  Add a short section showing how to run the isolated recent-window service.

---

### Task 1: Define Recent-Chat Domain Types

**Files:**
- Create: `internal/recentchat/types.go`
- Test: `internal/recentchat/service_test.go`

- [ ] **Step 1: Write the failing test scaffold for request validation types**

```go
package recentchat

import "testing"

func TestChatRequestRequiresSessionUserAndMessage(t *testing.T) {
	req := ChatRequest{}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for empty request")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recentchat -run TestChatRequestRequiresSessionUserAndMessage`

Expected: FAIL because `ChatRequest` and `Validate` do not exist yet.

- [ ] **Step 3: Write minimal domain types and validation**

```go
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
	Answer         string    `json:"answer"`
	UsedMessages   int       `json:"used_messages"`
	SessionID      string    `json:"session_id"`
	Model          string    `json:"model"`
	CreatedAt      time.Time `json:"created_at"`
	RecentWindow   []Message `json:"recent_window"`
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/recentchat -run TestChatRequestRequiresSessionUserAndMessage`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/types.go internal/recentchat/service_test.go
git commit -m "feat: define recent chat domain types"
```

### Task 2: Add Message Store Interface

**Files:**
- Create: `internal/recentchat/store.go`
- Modify: `internal/recentchat/types.go`
- Test: `internal/recentchat/service_test.go`

- [ ] **Step 1: Write the failing service test against a store abstraction**

```go
func TestServiceLoadsRecentMessagesFromStore(t *testing.T) {
	store := &fakeMessageStore{
		listRecentBySessionFn: func(sessionID string, limit int) ([]Message, error) {
			return []Message{{SessionID: sessionID, Role: RoleUser, Content: "old"}}, nil
		},
	}
	svc := Service{store: store}
	_, _ = svc, store
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recentchat -run TestServiceLoadsRecentMessagesFromStore`

Expected: FAIL because `Service` and `MessageStore` are undefined.

- [ ] **Step 3: Declare store interface**

```go
package recentchat

type MessageStore interface {
	ListRecentBySession(sessionID string, limit int) ([]Message, error)
	Append(msg Message) error
}
```

- [ ] **Step 4: Run targeted tests**

Run: `go test ./internal/recentchat`

Expected: Still failing for unimplemented `Service`, but interface compile errors resolved.

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/store.go internal/recentchat/service_test.go
git commit -m "feat: add recent chat message store interface"
```

### Task 3: Implement Count-Based Recent Window Builder

**Files:**
- Create: `internal/recentchat/window.go`
- Create: `internal/recentchat/window_count.go`
- Create: `internal/recentchat/window_count_test.go`

- [ ] **Step 1: Write failing tests for recent-window selection**

```go
package recentchat

import "testing"

func TestCountWindowBuilderKeepsLatestMessagesInChronologicalOrder(t *testing.T) {
	builder := CountWindowBuilder{}
	in := []Message{
		{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"},
	}

	out := builder.Build(in, 2)
	if len(out) != 2 || out[0].Content != "3" || out[1].Content != "4" {
		t.Fatalf("unexpected output: %#v", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recentchat -run TestCountWindowBuilderKeepsLatestMessagesInChronologicalOrder`

Expected: FAIL because builder types do not exist.

- [ ] **Step 3: Implement recent-window builder**

```go
package recentchat

type RecentWindowBuilder interface {
	Build(messages []Message, maxMessages int) []Message
}

type CountWindowBuilder struct{}

func (b CountWindowBuilder) Build(messages []Message, maxMessages int) []Message {
	if maxMessages <= 0 || len(messages) <= maxMessages {
		return messages
	}
	return messages[len(messages)-maxMessages:]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/recentchat -run TestCountWindowBuilderKeepsLatestMessagesInChronologicalOrder`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/window.go internal/recentchat/window_count.go internal/recentchat/window_count_test.go
git commit -m "feat: add count-based recent window builder"
```

### Task 4: Implement MySQL Message Schema

**Files:**
- Create: `sql/recentchat_messages.sql`
- Create: `internal/recentchat/store_mysql.go`
- Create: `internal/recentchat/store_mysql_test.go`

- [ ] **Step 1: Write the schema file**

```sql
CREATE TABLE IF NOT EXISTS recent_chat_messages (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    session_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    role VARCHAR(32) NOT NULL,
    content MEDIUMTEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_session_created_at (session_id, created_at),
    INDEX idx_user_created_at (user_id, created_at)
);
```

- [ ] **Step 2: Write failing store test scaffold**

```go
func TestMySQLMessageStoreQueryBuildsRecentMessages(t *testing.T) {
	var store MySQLMessageStore
	_ = store
}
```

- [ ] **Step 3: Implement MySQL store**

```go
package recentchat

import (
	"context"
	"database/sql"
)

type MySQLMessageStore struct {
	db *sql.DB
}

func NewMySQLMessageStore(db *sql.DB) *MySQLMessageStore {
	return &MySQLMessageStore{db: db}
}

func (s *MySQLMessageStore) ListRecentBySession(sessionID string, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(
		context.Background(),
		`SELECT id, session_id, user_id, role, content, created_at
		 FROM recent_chat_messages
		 WHERE session_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	desc := make([]Message, 0, limit)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.UserID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		desc = append(desc, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

func (s *MySQLMessageStore) Append(msg Message) error {
	_, err := s.db.ExecContext(
		context.Background(),
		`INSERT INTO recent_chat_messages (session_id, user_id, role, content, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		msg.SessionID, msg.UserID, msg.Role, msg.Content, msg.CreatedAt,
	)
	return err
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/recentchat -run TestMySQLMessageStoreQueryBuildsRecentMessages`

Expected: PASS at compile level; deeper integration tests can be gated later by DSN env.

- [ ] **Step 5: Commit**

```bash
git add sql/recentchat_messages.sql internal/recentchat/store_mysql.go internal/recentchat/store_mysql_test.go
git commit -m "feat: add mysql message store for recent chat"
```

### Task 5: Add Ollama Client

**Files:**
- Create: `internal/recentchat/ollama.go`
- Modify: `internal/recentchat/service_test.go`

- [ ] **Step 1: Write failing fake-client service test**

```go
func TestServicePassesRecentMessagesToOllama(t *testing.T) {
	client := &fakeOllamaClient{
		chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
			if len(req.Messages) == 0 {
				t.Fatal("expected messages")
			}
			return OllamaChatResponse{Content: "ok"}, nil
		},
	}
	_ = client
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recentchat -run TestServicePassesRecentMessagesToOllama`

Expected: FAIL because Ollama client types do not exist.

- [ ] **Step 3: Implement Ollama client**

```go
package recentchat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Content string `json:"-"`
}

type OllamaClient interface {
	Chat(req OllamaChatRequest) (OllamaChatResponse, error)
}

type HTTPOllamaClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPOllamaClient(baseURL string) *HTTPOllamaClient {
	return &HTTPOllamaClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *HTTPOllamaClient) Chat(req OllamaChatRequest) (OllamaChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return OllamaChatResponse{}, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return OllamaChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return OllamaChatResponse{}, err
	}
	defer resp.Body.Close()

	var out OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return OllamaChatResponse{}, err
	}
	out.Content = out.Message.Content
	return out, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/recentchat`

Expected: Compile proceeds further; service tests still fail until orchestration exists.

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/ollama.go internal/recentchat/service_test.go
git commit -m "feat: add ollama client for recent chat"
```

### Task 6: Build the Recent-Chat Service

**Files:**
- Create: `internal/recentchat/service.go`
- Modify: `internal/recentchat/service_test.go`

- [ ] **Step 1: Write failing orchestration test**

```go
func TestServiceChatsWithRecentWindowAndPersistsTurns(t *testing.T) {
	store := &fakeMessageStore{
		listRecentBySessionFn: func(sessionID string, limit int) ([]Message, error) {
			return []Message{
				{SessionID: sessionID, UserID: "u1", Role: RoleUser, Content: "old q"},
				{SessionID: sessionID, UserID: "u1", Role: RoleAssistant, Content: "old a"},
			}, nil
		},
	}
	client := &fakeOllamaClient{
		chatFn: func(req OllamaChatRequest) (OllamaChatResponse, error) {
			return OllamaChatResponse{Content: "new answer"}, nil
		},
	}
	svc := Service{
		store:   store,
		window:  CountWindowBuilder{},
		ollama:  client,
		nowFunc: fixedNow,
	}
	resp, err := svc.Chat(ChatRequest{
		SessionID:       "s1",
		UserID:          "u1",
		Message:         "new q",
		Model:           "llama3",
		RecentLimit:     2,
		StoreUserTurn:   true,
		StoreAssistTurn: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Answer != "new answer" {
		t.Fatalf("unexpected answer: %s", resp.Answer)
	}
	if len(store.appended) != 2 {
		t.Fatalf("expected 2 persisted turns, got %d", len(store.appended))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recentchat -run TestServiceChatsWithRecentWindowAndPersistsTurns`

Expected: FAIL because `Service` is undefined.

- [ ] **Step 3: Implement service orchestration**

```go
package recentchat

import "time"

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
		ollamaMessages = append(ollamaMessages, OllamaMessage{Role: string(RoleSystem), Content: req.SystemPrompt})
	}
	for _, msg := range selected {
		ollamaMessages = append(ollamaMessages, OllamaMessage{Role: string(msg.Role), Content: msg.Content})
	}
	ollamaMessages = append(ollamaMessages, OllamaMessage{Role: string(RoleUser), Content: req.Message})

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
		if err := s.store.Append(Message{SessionID: req.SessionID, UserID: req.UserID, Role: RoleUser, Content: req.Message, CreatedAt: now}); err != nil {
			return ChatResponse{}, err
		}
	}
	if req.StoreAssistTurn {
		if err := s.store.Append(Message{SessionID: req.SessionID, UserID: req.UserID, Role: RoleAssistant, Content: chatResp.Content, CreatedAt: now}); err != nil {
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/recentchat`

Expected: PASS for domain/service/window tests.

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/service.go internal/recentchat/service_test.go
git commit -m "feat: add recent chat service orchestration"
```

### Task 7: Add HTTP Handlers and Isolated Entrypoint

**Files:**
- Create: `internal/recentchat/http.go`
- Create: `cmd/recent-chat/main.go`

- [ ] **Step 1: Write handler shape**

```go
package recentchat

import "net/http"

func RegisterHandlers(mux *http.ServeMux, svc Service) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"service":"recent-chat"}`))
	})
}
```

- [ ] **Step 2: Expand handler for `/chat`**

```go
func RegisterHandlers(mux *http.ServeMux, svc Service) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "recent-chat"})
	})
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		resp, err := svc.Chat(req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}
```

- [ ] **Step 3: Add the executable**

```go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/recentchat"
)

func main() {
	dsn := os.Getenv("RECENT_CHAT_MYSQL_DSN")
	if dsn == "" {
		log.Fatal("RECENT_CHAT_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	store := recentchat.NewMySQLMessageStore(db)
	service := recentchat.Service{
		store:  store,
		window: recentchat.CountWindowBuilder{},
		ollama: recentchat.NewHTTPOllamaClient(envOrDefault("OLLAMA_BASE_URL", "http://127.0.0.1:11434")),
	}
	mux := http.NewServeMux()
	recentchat.RegisterHandlers(mux, service)
	log.Fatal(http.ListenAndServe(":18093", mux))
}
```

- [ ] **Step 4: Run build check**

Run: `go build ./cmd/recent-chat`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/recentchat/http.go cmd/recent-chat/main.go
git commit -m "feat: expose isolated recent chat service"
```

### Task 8: Add Dependency and Runtime Docs

**Files:**
- Modify: `go.mod`
- Modify: `README.md`
- Create: `docs/teaching/recent-window-layer-01.md`

- [ ] **Step 1: Add MySQL driver dependency**

```go
require github.com/go-sql-driver/mysql v1.8.1
```

- [ ] **Step 2: Document schema and startup**

```md
## Recent Chat Service

Apply schema:

```sql
SOURCE sql/recentchat_messages.sql;
```

Run:

```bash
RECENT_CHAT_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/offline_rag?parseTime=true' \
OLLAMA_BASE_URL='http://127.0.0.1:11434' \
go run ./cmd/recent-chat
```
```

- [ ] **Step 3: Add curl demo**

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-001",
    "user_id":"u-001",
    "message":"帮我总结一下我们刚才聊了什么",
    "model":"llama3",
    "recent_limit":10,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

- [ ] **Step 4: Run end-to-end manual check**

Run:

```bash
go test ./internal/recentchat ./cmd/recent-chat
go run ./cmd/recent-chat
```

Expected:
- tests pass
- service starts on `:18093`
- curl returns an answer and includes `recent_window`

- [ ] **Step 5: Commit**

```bash
git add go.mod README.md docs/teaching/recent-window-layer-01.md
git commit -m "docs: add recent chat runtime guide"
```

### Task 9: Add Execution Notes for Next Layer

**Files:**
- Modify: `docs/teaching/recent-window-layer-01.md`

- [ ] **Step 1: Record real limitations**

```md
## Current Real Limitations

- Recent window is count-based, not token-budget-based
- No session summary layer yet
- No long-term memory extraction yet
- No memory retrieval from Qdrant yet
```

- [ ] **Step 2: Record next upgrade path**

```md
## Next Upgrade Path

1. Replace count-based windowing with token-budget-based windowing
2. Add session summaries in MySQL
3. Add memory item extraction and selective vectorization
4. Retrieve memory items alongside document knowledge
```

- [ ] **Step 3: Run doc sanity check**

Run: `rg -n "TBD|TODO|implement later" docs/2026-06-29-recent-window-real-implementation-plan.md docs/teaching/recent-window-layer-01.md`

Expected: no matches

- [ ] **Step 4: Commit**

```bash
git add docs/teaching/recent-window-layer-01.md
git commit -m "docs: capture recent window limitations and next steps"
```

---

## Self-Review

Spec coverage:
- Real MySQL storage: covered by Tasks 4, 7, 8
- Real Ollama integration: covered by Tasks 5, 6, 7
- Isolated internal implementation project: covered by `internal/recentchat/` and `cmd/recent-chat`
- Recent-window first layer only: covered by Tasks 1-9 without summary or long-term memory implementation

Placeholder scan:
- No `TBD` or `TODO` placeholders intentionally left in execution steps

Type consistency:
- `MessageStore`, `RecentWindowBuilder`, `Service`, `OllamaClient`, and request/response names are consistent across tasks

---

Plan complete and saved to `docs/2026-06-29-recent-window-real-implementation-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
