# Three Token Context Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 依次实现真实模板渲染、模板 token 开销和上下文预算规划。

**Architecture:** 新建 `internal/promptbudget`，把模板渲染、token 对比和预算计算拆成三个独立文件。单个 `cmd/prompt-budget-demo` 在三个 commit 中逐步获得新能力，复用现有 Ollama Show client 与 tokenizer counter。

**Tech Stack:** Go、`text/template`、Ollama `/api/show`、现有本地 tokenizer、Go testing

## Global Constraints

- 只修改当前仓库。
- 不修改 `/chat`、数据库或外部服务状态。
- 每个 Task 独立 review、验证、commit。
- 禁止 `git push`。
- 所有低风险操作及影响记录到 `docs/teaching/00-batch-operation-log.md`。

---

### Task 1: 真实模板渲染

**Files:**
- Create: `internal/promptbudget/render.go`
- Create: `internal/promptbudget/render_test.go`
- Create: `cmd/prompt-budget-demo/main.go`
- Create: `docs/teaching/prompt-template-render-sop.md`
- Create: `docs/teaching/00-batch-operation-log.md`

**Produces:** `Render(templateText string, system string, prompt string) (string, error)`

- [x] 写测试：有 system 时渲染 system/user/assistant 包装，无 system 时省略 system 块，非法模板返回错误
- [x] 运行 `go test ./internal/promptbudget` 并确认 RED
- [x] 实现 `Render` 和只打印 rendered prompt 的命令
- [x] 运行命令、review、全量测试
- [x] 提交 `feat: render Ollama prompt template`

### Task 2: 模板 token 开销

**Files:**
- Create: `internal/promptbudget/count.go`
- Create: `internal/promptbudget/count_test.go`
- Modify: `cmd/prompt-budget-demo/main.go`
- Create: `docs/teaching/prompt-template-token-overhead-sop.md`
- Modify: `docs/teaching/00-batch-operation-log.md`

**Consumes:** `Render(...)`、`tokenizerdemo.LoadCounter(...)`

**Produces:** `CompareTokens(counter TextTokenCounter, system string, prompt string, rendered string) (TokenComparison, error)`

- [x] 写测试：统计正文 token、渲染 token 和差值，计数错误向上传递
- [x] 运行目标测试并确认 RED
- [x] 增加 `--tokenizer` 和 token 对比输出
- [x] 运行命令、review、全量测试
- [x] 提交 `feat: compare prompt template token overhead`

### Task 3: 上下文预算规划

**Files:**
- Create: `internal/promptbudget/budget.go`
- Create: `internal/promptbudget/budget_test.go`
- Modify: `cmd/prompt-budget-demo/main.go`
- Create: `docs/teaching/context-budget-plan-sop.md`
- Modify: `docs/teaching/00-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`

**Consumes:** Ollama `ContextLength`、`TokenComparison.RenderedTokens`

**Produces:** `Plan(contextLimit int, fixedInputTokens int, outputReserve int) (BudgetPlan, error)`

- [x] 写测试：正确计算 history budget，输入加输出超限时报错
- [x] 运行目标测试并确认 RED
- [x] 增加 `--output-reserve` 和预算输出
- [x] 运行命令、review、全量测试
- [x] 提交 `feat: plan context token budget`
