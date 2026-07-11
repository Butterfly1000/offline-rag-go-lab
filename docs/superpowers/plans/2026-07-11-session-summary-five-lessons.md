# Session Summary Five Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成第 14-18 节，使 Session Summary 从连续旧消息选择、增量生成、MySQL version 更新到 `/chat` 使用形成真实闭环。

**Architecture:** `internal/sessionsummary` 通过小接口组合 selector、token counter、generator、store 和 updater，不依赖 `recentchat`。第 18 节由 `recentchat` 注入这些能力，并使用固定 summary token reserve 打破 summary 长度与 recent 起点的循环依赖。

**Tech Stack:** Go 1.23、MySQL、Ollama `/api/chat`、本地 Qwen tokenizer、Go testing

## Global Constraints

- 只修改 `/offline-rag-go-lab`。
- 每节 RED -> GREEN -> 实践/SOP -> review -> 独立 commit。
- 不执行 `git push`。
- 不实现 long-term memory 或 Qdrant。
- SQL schema、凭据和外部状态变更前停止询问。
- 文档路径使用 `/offline-rag-go-lab/...`。

---

### Task 14: 驱逐前缀选择与 Token 统计

**Files:**
- Create: `internal/sessionsummary/select.go`
- Create: `internal/sessionsummary/select_test.go`
- Create: `internal/sessionsummary/token.go`
- Create: `internal/sessionsummary/token_test.go`
- Create: `cmd/summary-selection-demo/main.go`
- Create: `docs/teaching/session-summary-selection-sop.md`

**Produces:**

```go
type SourceMessage struct { ID int64; Role string; Content string }
type PrefixSelection struct {
    Unsummarized []SourceMessage
    Evicted []SourceMessage
    UnsummarizedTokens int
    EvictedTokens int
    NextWatermark int64
}
func SelectPrefix(messages []SourceMessage, lastMessageID, recentStartID int64, counter MessageTokenCounter) (PrefixSelection, error)
```

- [x] Write tests for old-message filtering, recent boundary exclusion, empty recent window, ID gaps, no eviction, unordered/duplicate/invalid IDs, missing recent start, and counter errors.
- [x] Run `go test ./internal/sessionsummary -run 'TestSelectPrefix'` and confirm RED.
- [x] Implement minimal selector and Qwen formatted message counter adapter.
- [x] Run GREEN tests and a command showing watermark `20`, IDs `21..26`, recent start `25`, evicted `21..24`, next watermark `24`.
- [x] Write SOP, review, run full gates, commit `feat: select summary message prefix`.

### Task 15: 真实 Ollama 增量摘要生成

**Files:**
- Create: `internal/sessionsummary/prompt.go`
- Create: `internal/sessionsummary/prompt_test.go`
- Create: `internal/sessionsummary/generator.go`
- Create: `internal/sessionsummary/generator_test.go`
- Modify: `internal/recentchat/ollama.go`
- Create: `internal/recentchat/ollama_generate_test.go`
- Create: `cmd/summary-generate-demo/main.go`
- Create: `docs/teaching/session-summary-generation-sop.md`

**Produces:**

```go
type TextGenerator interface {
    GenerateText(model, system, prompt string, maxTokens int) (string, error)
}
type Generator struct { /* TextGenerator */ }
func (g Generator) Update(model, previous string, messages []SourceMessage, maxTokens int) (string, error)
```

- [x] Write tests for prompt sections/order/escaping, empty inputs, model/output validation, error propagation, trimming, and empty model output.
- [x] Confirm RED, implement prompt builder and generator.
- [x] Add `HTTPOllamaClient.GenerateText` using `/api/chat` with system/user messages and `num_predict`.
- [x] Run real `qwen:7b` command with old summary and evicted messages; capture result in SOP.
- [x] Review/full gates, commit `feat: generate incremental session summary`.

### Task 16: MySQL Summary Store 与 Version 更新

**Files:**
- Create: `internal/sessionsummary/store.go`
- Create: `internal/sessionsummary/store_test.go`
- Create: `internal/sessionsummary/store_mysql.go`
- Create: `internal/sessionsummary/store_mysql_test.go`
- Create: `cmd/summary-store-demo/main.go`
- Create: `docs/teaching/session-summary-store-sop.md`

**Produces:**

```go
var ErrVersionConflict error
type SummaryStore interface {
    Get(sessionID, userID string) (SessionSummary, bool, error)
    Save(next SessionSummary, expectedVersion int64) (SessionSummary, error)
}
```

- [x] Write RED tests for missing/existing Get, first insert, versioned update, zero affected conflict, content/watermark validation and errors.
- [x] Implement store over a testable `SummaryQueries` boundary and concrete MySQL queries.
- [x] Add real command reading config and performing Get/Save on a dedicated test session.
- [x] Stop and request permission before executing `sql/session_summaries.sql` or the real command.
- [x] After approval, run schema + real store practice, document rows/version; review/full gates, commit `feat: persist versioned session summaries`.

### Task 17: 滚动摘要更新服务

**Files:**
- Create: `internal/sessionsummary/update.go`
- Create: `internal/sessionsummary/update_test.go`
- Create: `internal/sessionsummary/source_mysql.go`
- Create: `internal/sessionsummary/source_mysql_test.go`
- Create: `cmd/summary-update-demo/main.go`
- Create: `docs/teaching/session-summary-update-sop.md`

**Produces:**

```go
type MessageSource interface {
    ListAfter(sessionID, userID string, lastMessageID int64) ([]SourceMessage, error)
}
type UpdateRequest struct { SessionID, UserID, Model string; RecentStartID int64; MaxOutputTokens int }
type UpdateResult struct { Updated bool; Decision TriggerDecision; Summary SessionSummary; Selection PrefixSelection }
func (s UpdateService) Update(req UpdateRequest) (UpdateResult, error)
```

- [x] RED tests for no trigger, first summary, rolling summary, generator failure, save failure/conflict, and watermark preservation.
- [x] Implement MySQL message source and updater orchestration.
- [x] Run real command against dedicated session using MySQL, tokenizer and Ollama; verify version/watermark.
- [x] Write SOP, review/full gates, commit `feat: orchestrate rolling session summaries`.

### Task 18: Summary 接入自动预算 `/chat`

**Files:**
- Modify: `internal/recentchat/types.go`
- Modify: `internal/recentchat/service.go`
- Modify: `internal/recentchat/service_test.go`
- Modify: `internal/recentchat/http.go`
- Modify: `cmd/recent-chat/main.go`
- Create: `docs/teaching/recent-chat-session-summary-sop.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`

**Produces:** request/response summary fields and optional summary dependencies in `Service`.

- [ ] RED tests for request validation, conservative reserve, update/read ordering, combined system prompt, final budget invariant, old-request compatibility, dependency/error handling, and response fields.
- [ ] Implement `use_session_summary` only with automatic budget; configure summary input reserve/output limit in `recent-chat`.
- [ ] Verify actual formatted summary tokens do not exceed reserve and final available history is not below conservative selection budget.
- [ ] Run real two-request curl on a dedicated session proving summary create/use/version/watermark; document database and response evidence.
- [ ] Update learning status/backlog, run complete audit, commit `feat: integrate session summary into chat`.

## Completion Audit

- [ ] Exactly five implementation commits after this plan.
- [ ] Five SOPs and practical evidence exist.
- [ ] Full test/race/vet/build/diff checks pass.
- [ ] Real curl proves Session Summary closed loop.
- [ ] No credentials or local assets tracked.
- [ ] Worktree clean; no push.
