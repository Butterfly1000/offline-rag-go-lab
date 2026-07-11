# Session Summary Trigger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现第 13 节 session summary 数据结构和可解释触发策略，并提供 SQL、命令和教学 SOP。

**Architecture:** 新建独立 `internal/sessionsummary`，把确定性触发逻辑与 recent-chat、Ollama、MySQL 解耦。SQL 只定义未来 store 的持久化边界；命令直接调用同一生产逻辑，不复制判断公式。

**Tech Stack:** Go 1.23、Go testing、MySQL 8 兼容 DDL、标准库 `flag`

## Global Constraints

- 只修改 `/offline-rag-go-lab`。
- 不执行 SQL、不连接或修改 MySQL、Qdrant、Ollama。
- 不修改现有 `/chat` 行为。
- 使用 TDD，先观察目标测试因 API 缺失而失败。
- 提交前执行 review、全量 test、race、vet、build 和 diff check。
- 创建独立实现 commit，不执行 `git push`。
- 新文档路径使用 `/offline-rag-go-lab/...`，不写完整本机路径。

---

### Task 13: Session Summary 数据结构与触发策略

**Files:**
- Create: `internal/sessionsummary/types.go`
- Create: `internal/sessionsummary/trigger.go`
- Create: `internal/sessionsummary/trigger_test.go`
- Create: `cmd/summary-trigger-demo/main.go`
- Create: `sql/session_summaries.sql`
- Create: `docs/teaching/session-summary-trigger-sop.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`

**Interfaces:**
- Produces: `SessionSummary`
- Produces: `TriggerInput`
- Produces: `TriggerDecision`
- Produces: `NewTriggerPolicy(minMessages int, minTokens int) (TriggerPolicy, error)`
- Produces: `TriggerPolicy.Decide(input TriggerInput) (TriggerDecision, error)`

- [ ] **Step 1: Write the failing trigger tests**

```go
func TestTriggerPolicyRequiresEvictedMessages(t *testing.T) {
    policy, _ := NewTriggerPolicy(8, 2048)
    decision, err := policy.Decide(TriggerInput{
        UnsummarizedMessages: 10,
        UnsummarizedTokens:   5000,
        EvictedMessages:      0,
    })
    // assert no error, false, ReasonNoEvictedMessages
}

func TestTriggerPolicyTriggersAtMessageThreshold(t *testing.T) {
    policy, _ := NewTriggerPolicy(8, 2048)
    decision, err := policy.Decide(TriggerInput{
        UnsummarizedMessages: 8,
        UnsummarizedTokens:   1000,
        EvictedMessages:      2,
    })
    // assert true, ReasonMessageThreshold
}

func TestTriggerPolicyTriggersAtTokenThreshold(t *testing.T) {
    policy, _ := NewTriggerPolicy(8, 2048)
    decision, err := policy.Decide(TriggerInput{
        UnsummarizedMessages: 3,
        UnsummarizedTokens:   3000,
        EvictedMessages:      1,
    })
    // assert true, ReasonTokenThreshold
}
```

Also test both thresholds, below threshold, non-positive policy values, negative input, and `EvictedMessages > UnsummarizedMessages`.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/sessionsummary
```

Expected: compile failure because `TriggerPolicy`, `TriggerInput`, and reason constants do not exist.

- [ ] **Step 3: Implement types and minimal policy**

```go
type SessionSummary struct {
    SessionID    string
    UserID       string
    Content      string
    LastMessageID int64
    Version      int64
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type TriggerInput struct {
    UnsummarizedMessages int
    UnsummarizedTokens   int
    EvictedMessages      int
}

type TriggerDecision struct {
    ShouldSummarize bool
    Reason          TriggerReason
}

func NewTriggerPolicy(minMessages int, minTokens int) (TriggerPolicy, error)
func (p TriggerPolicy) Decide(input TriggerInput) (TriggerDecision, error)
```

Decision precedence:

```text
validate
-> no evicted
-> both thresholds
-> message threshold
-> token threshold
-> below threshold
```

- [ ] **Step 4: Verify GREEN**

Run:

```bash
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./internal/sessionsummary
```

Expected: all trigger tests pass.

- [ ] **Step 5: Add SQL and practical command**

The SQL table must contain:

```sql
PRIMARY KEY (session_id, user_id)
```

and `content`, `last_message_id`, `version`, `created_at`, `updated_at`.

The command accepts:

```text
--messages
--tokens
--evicted
--min-messages
--min-tokens
```

Run four scenarios:

```bash
go run ./cmd/summary-trigger-demo --messages 10 --tokens 5000 --evicted 0
go run ./cmd/summary-trigger-demo --messages 8 --tokens 1000 --evicted 2
go run ./cmd/summary-trigger-demo --messages 3 --tokens 3000 --evicted 1
go run ./cmd/summary-trigger-demo --messages 3 --tokens 1000 --evicted 1
```

Expected reasons: `no_evicted_messages`, `message_threshold`, `token_threshold`, `below_threshold`.

- [ ] **Step 6: Write SOP and update progress**

The SOP explains:

- why `last_message_id` is a watermark
- why eviction is required before threshold evaluation
- why message/token thresholds use OR
- how to inspect but not automatically execute the SQL
- which next section will calculate real input and call Ollama

Update learning status to mark only “summary structure and trigger” complete. Add non-blocking optimizations to the backlog.

- [ ] **Step 7: Review, verify, and commit**

Run:

```bash
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test ./...
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go test -race ./internal/sessionsummary
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go vet ./...
env GOCACHE=$PWD/.cache/go-build GOSUMDB=off go build ./cmd/...
git diff --check
```

Review schema idempotency, invalid-input handling, threshold precedence, docs accuracy, ignored files, and sensitive information.

Commit: `feat: add session summary trigger policy`
