# Five Token Chat Integration Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现第 08 到第 12 节，使 `recent-chat` 能按 Qwen 消息格式计算 token、自动规划历史预算并通过 `/chat` 返回预算明细。

**Architecture:** 新建无外部状态依赖的 `internal/chatprompt` 负责 Qwen ChatML 格式与计数；`internal/promptbudget` 组合模型 context provider 和固定输入计数；`internal/recentchat` 只负责编排 MySQL 历史、预算和 Ollama。现有 count/manual API 保留，自动模式显式启用。

**Tech Stack:** Go 1.23、现有本地 tokenizer、Ollama `/api/show` 与 `/api/chat`、MySQL message store、Go testing

## Global Constraints

- 只修改 `/offline-rag-go-lab`，不修改数据库 schema、Qdrant 或 Ollama 模型文件。
- 不读取、提交或输出 `config/recent-chat.env` 中的凭据。
- 每节遵循 RED -> GREEN -> review -> verification -> 独立 commit。
- 每节提供教学 SOP 和可执行实践入口。
- 所有本批次操作和影响记录到 `docs/teaching/00-five-token-section-operation-log.md`。
- 不执行 `git push`。
- 新文档使用 `/offline-rag-go-lab/...` 形式的仓库路径。

---

### Task 08: Qwen 历史消息格式化

**Files:**
- Create: `internal/chatprompt/qwen.go`
- Create: `internal/chatprompt/qwen_test.go`
- Create: `cmd/message-format-demo/main.go`
- Create: `docs/teaching/qwen-message-format-sop.md`
- Create: `docs/teaching/00-five-token-section-operation-log.md`

**Interfaces:**
- Produces: `Message{Role string, Content string}`
- Produces: `QwenFormatter.FormatMessage(Message) (string, error)`
- Produces: `QwenFormatter.AssistantPrefix() string`

- [x] **Step 1: Write failing formatter tests**

```go
func TestQwenFormatterFormatsRoleContentAndBoundaries(t *testing.T) {
    got, err := (QwenFormatter{}).FormatMessage(Message{Role: "user", Content: "你好"})
    want := "<|im_start|>user\n你好<|im_end|>\n"
    // assert err == nil and got == want
}

func TestQwenFormatterRejectsUnknownRole(t *testing.T) {
    _, err := (QwenFormatter{}).FormatMessage(Message{Role: "unknown", Content: "x"})
    // assert err contains "unsupported message role"
}
```

- [x] **Step 2: Verify RED**

Run: `env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/chatprompt`

Expected: compile failure because `QwenFormatter` does not exist.

- [x] **Step 3: Implement the minimal formatter**

```go
type Message struct {
    Role    string
    Content string
}

type QwenFormatter struct{}

func (QwenFormatter) FormatMessage(message Message) (string, error)
func (QwenFormatter) AssistantPrefix() string
```

Allow only `system`, `user`, `assistant`, and `tool`; use `strings.Builder` so the wrapper is visible and testable.

- [x] **Step 4: Add the practical command and SOP**

Run: `go run ./cmd/message-format-demo --role user --content '你好，解释 token。'`

Expected output includes the role marker, content, end marker, and real newline boundaries.

- [x] **Step 5: Review, verify, and commit**

Run formatter tests, `go test ./...`, `go vet ./internal/chatprompt ./cmd/message-format-demo`, `go build ./cmd/...`, and `git diff --check`.

Commit: `feat: format Qwen chat messages`

### Task 09: 完整对话 prompt 计数

**Files:**
- Modify: `internal/chatprompt/qwen.go`
- Modify: `internal/chatprompt/qwen_test.go`
- Create: `internal/chatprompt/count.go`
- Create: `internal/chatprompt/count_test.go`
- Create: `cmd/conversation-token-demo/main.go`
- Create: `docs/teaching/conversation-token-count-sop.md`
- Modify: `docs/teaching/00-five-token-section-operation-log.md`

**Interfaces:**
- Consumes: `QwenFormatter.FormatMessage` and `AssistantPrefix`
- Produces: `QwenFormatter.Render([]Message, bool) (string, error)`
- Produces: `NewTokenCounter(TextTokenCounter, QwenFormatter) TokenCounter`
- Produces: `TokenCounter.Count([]Message, bool) (TokenUsage, error)`

- [x] **Step 1: Write failing render and count tests**

```go
func TestQwenFormatterRendersConversationAndAssistantPrefix(t *testing.T) {
    messages := []Message{{Role: "system", Content: "规则"}, {Role: "user", Content: "问题"}}
    got, err := (QwenFormatter{}).Render(messages, true)
    // assert ordered message wrappers and final assistant prefix
}

func TestTokenCounterCountsRenderedConversationOnce(t *testing.T) {
    raw := &recordingCounter{count: 17}
    usage, err := NewTokenCounter(raw, QwenFormatter{}).Count([]Message{{Role: "user", Content: "问题"}}, true)
    // assert one CountText call, rendered input, and TotalTokens == 17
}
```

- [x] **Step 2: Verify RED**

Run: `env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/chatprompt`

Expected: compile failure because `Render`, `TokenCounter`, and `TokenUsage` do not exist.

- [x] **Step 3: Implement full rendering and counting**

```go
type TextTokenCounter interface {
    CountText(string) (int, []string, []int, error)
}

type TokenUsage struct {
    Rendered    string
    TotalTokens int
}

func (f QwenFormatter) Render(messages []Message, includeAssistantPrefix bool) (string, error)
func NewTokenCounter(counter TextTokenCounter, formatter QwenFormatter) TokenCounter
func (c TokenCounter) Count(messages []Message, includeAssistantPrefix bool) (TokenUsage, error)
```

Count the complete rendered string once; wrap formatter/tokenizer errors with operation context.

- [x] **Step 4: Add the tokenizer-backed command and SOP**

Run: `go run ./cmd/conversation-token-demo --system '你是 Go 助手。' --history-user '我叫小黄。' --history-assistant '记住了。' --prompt '我叫什么？'`

Expected output includes rendered conversation and total prompt tokens.

- [x] **Step 5: Review, verify, and commit**

Run target/full tests, race test for `internal/chatprompt`, vet, all command builds, real demo, and `git diff --check`.

Commit: `feat: count complete Qwen conversation tokens`

### Task 10: 模板感知的 recent window

**Files:**
- Modify: `internal/recentchat/window_token_budget.go`
- Modify: `internal/recentchat/window_token_budget_test.go`
- Modify: `cmd/recent-chat/main.go`
- Create: `docs/teaching/recent-window-template-token-sop.md`
- Modify: `docs/teaching/00-five-token-section-operation-log.md`

**Interfaces:**
- Consumes: `chatprompt.QwenFormatter.FormatMessage`
- Preserves: `NewTokenBudgetWindowBuilder(counter)` as legacy content-only behavior
- Produces: `NewFormattedTokenBudgetWindowBuilder(counter, formatter) TokenBudgetWindowBuilder`

- [x] **Step 1: Write failing strict formatted-window tests**

```go
func TestFormattedTokenWindowCountsRoleAndBoundaries(t *testing.T) {
    // Fake counter assigns costs to fully formatted user/assistant messages.
    // Assert the selected suffix and used token count come from formatted text.
}

func TestFormattedTokenWindowDoesNotForceOversizedNewestMessage(t *testing.T) {
    // Newest formatted message costs 10 with budget 9.
    // Assert empty selection and zero used tokens.
}
```

- [x] **Step 2: Verify RED**

Run: `env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/recentchat -run 'TestFormattedTokenWindow'`

Expected: compile failure because `NewFormattedTokenBudgetWindowBuilder` does not exist.

- [x] **Step 3: Implement the formatted strict mode**

```go
type TokenBudgetWindowBuilder struct {
    counter   TextTokenCounter
    formatter *chatprompt.QwenFormatter
    strict    bool
}

func NewFormattedTokenBudgetWindowBuilder(counter TextTokenCounter, formatter chatprompt.QwenFormatter) TokenBudgetWindowBuilder
```

Legacy constructor continues counting `Content` and preserving the existing oversized-newest behavior. The formatted constructor counts the complete wrapper and never exceeds the budget.

- [x] **Step 4: Wire the real service entry and write SOP**

Change `cmd/recent-chat` to construct the formatted builder. Use target tests as the no-database practice; document the existing curl with `recent_token_budget` for real MySQL/Ollama verification.

- [x] **Step 5: Review, verify, and commit**

Run target/full/race tests, vet, command builds, `git diff --check`, and inspect compatibility tests.

Commit: `feat: count formatted recent message tokens`

### Task 11: 自动历史预算规划

**Files:**
- Create: `internal/promptbudget/automatic.go`
- Create: `internal/promptbudget/automatic_test.go`
- Modify: `internal/recentchat/ollama.go`
- Modify: `internal/recentchat/ollama_show_test.go`
- Create: `cmd/automatic-budget-demo/main.go`
- Create: `docs/teaching/automatic-history-budget-sop.md`
- Modify: `docs/teaching/00-five-token-section-operation-log.md`

**Interfaces:**
- Consumes: `chatprompt.TokenCounter.Count` and `promptbudget.Plan`
- Produces: `ContextProvider.ContextLength(model string) (int, error)`
- Produces: `NewAutomaticPlanner(ContextProvider, ConversationCounter) AutomaticPlanner`
- Produces: `AutomaticPlanner.Plan(model string, fixed []chatprompt.Message, outputReserve int) (AutomaticPlan, error)`
- Produces: `HTTPOllamaClient.ContextLength(model string) (int, error)`

- [ ] **Step 1: Write failing automatic planner tests**

```go
func TestAutomaticPlannerUsesModelContextAndCompleteFixedPrompt(t *testing.T) {
    // Context = 32768, fixed count = 88, reserve = 2048.
    // Assert available history = 30632 and preserve all breakdown fields.
}

func TestAutomaticPlannerPropagatesContextAndCounterErrors(t *testing.T) {
    // Assert no fallback estimate is returned.
}
```

- [ ] **Step 2: Verify RED**

Run: `env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/promptbudget`

Expected: compile failure because `AutomaticPlanner` does not exist.

- [ ] **Step 3: Implement the planner and Ollama adapter**

```go
type ContextProvider interface {
    ContextLength(model string) (int, error)
}

type ConversationCounter interface {
    Count([]chatprompt.Message, bool) (chatprompt.TokenUsage, error)
}

type AutomaticPlan struct {
    BudgetPlan
    RenderedFixedPrompt string
}
```

`AutomaticPlanner.Plan` calls the provider, counts fixed messages with assistant prefix, then delegates arithmetic to existing `Plan`.

- [ ] **Step 4: Add real Ollama/tokenizer command and SOP**

Run: `go run ./cmd/automatic-budget-demo --model qwen:7b --system '你是 Go 助手。' --prompt '解释 recent window。' --output-reserve 2048`

Expected output shows context limit, fixed prompt tokens, output reserve, and available history tokens.

- [ ] **Step 5: Review, verify, and commit**

Run target/full/race tests, vet, all command builds, real command if Ollama is available, and `git diff --check`.

Commit: `feat: plan recent history budget automatically`

### Task 12: `/chat` 自动预算和可观测性

**Files:**
- Modify: `internal/recentchat/types.go`
- Modify: `internal/recentchat/service.go`
- Modify: `internal/recentchat/service_test.go`
- Modify: `internal/recentchat/http.go`
- Modify: `cmd/recent-chat/main.go`
- Create: `docs/teaching/recent-chat-automatic-token-budget-sop.md`
- Modify: `docs/teaching/recent-window-runtime-sop.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`
- Modify: `docs/teaching/00-five-token-section-operation-log.md`

**Interfaces:**
- Consumes: `promptbudget.AutomaticPlanner.Plan`
- Produces request fields: `AutoTokenBudget bool`, `OutputTokenReserve int`
- Produces response fields: `BudgetMode`, `ContextLimit`, `FixedInputTokens`, `OutputTokenReserve`, `AvailableRecentTokens`
- Produces: `NewServiceWithAutomaticBudget(...) Service`

- [ ] **Step 1: Write failing validation and service tests**

```go
func TestChatRequestRejectsConflictingAutomaticAndManualBudgets(t *testing.T) {}
func TestChatRequestRequiresOutputReserveForAutomaticBudget(t *testing.T) {}
func TestServiceUsesAutomaticBudgetAndReturnsBreakdown(t *testing.T) {
    // Fake planner returns context 32768, fixed 88, reserve 2048, history 30632.
    // Assert selected history uses 30632, Ollama receives selected history,
    // and response exposes the same budget breakdown.
}
func TestServiceKeepsLegacyCountAndManualModes(t *testing.T) {}
```

- [ ] **Step 2: Verify RED**

Run: `env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/recentchat`

Expected: compile failure because new request/response fields and constructor do not exist.

- [ ] **Step 3: Implement service orchestration**

```go
type AutomaticBudgetPlanner interface {
    Plan(model string, fixed []chatprompt.Message, outputReserve int) (promptbudget.AutomaticPlan, error)
}
```

The service selects exactly one mode:

```text
auto_token_budget -> automatic
else recent_token_budget > 0 -> manual
else -> count
```

Automatic mode plans before selecting history, uses default fetch limit `50` only when `recent_limit <= 0`, and returns all breakdown fields.

- [ ] **Step 4: Wire main, document curl, and update learning status**

Run the service and send:

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"s-auto-token-001",
    "user_id":"u-001",
    "message":"根据历史说明这个项目的语言。",
    "model":"qwen:7b",
    "recent_limit":50,
    "auto_token_budget":true,
    "output_token_reserve":2048,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

Document how to read each response field and clearly state whether the real curl was executed.

- [ ] **Step 5: Perform completion audit and commit**

Run:

```bash
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./...
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test -race ./internal/chatprompt ./internal/promptbudget ./internal/recentchat
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go vet ./...
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go build ./cmd/...
git diff --check
```

Review all changes since the design baseline, verify exactly five section commits, inspect ignored/sensitive files, and confirm no push occurred.

Commit: `feat: integrate automatic token budget into chat`
