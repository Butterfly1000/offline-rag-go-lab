# Tokenizer Load Once Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 tokenizer 在程序启动时加载一次，并让 demo 能通过命令行接收任意待计算文本。

**Architecture:** `tokenizerdemo.Counter` 保存已经加载完成的 tokenizer 实例，`CountText` 只负责编码。两个程序入口负责处理初始化错误；demo 用 `--text` 接收实践文本。

**Tech Stack:** Go、`github.com/sugarme/tokenizer`、Go `flag`、Go `testing`

## Global Constraints

- 本小节不解析 tokenizer 内部配置。
- 本小节不实现 chat template。
- 必须保留 `recent-chat` 的现有 token-budget 行为。
- 完成后先 review 和验证，再单独 commit，不 push。

---

### Task 1: 启动时加载 tokenizer，并支持任意文本

**Files:**
- Create: `internal/tokenizerdemo/tokenizer_test.go`
- Modify: `internal/tokenizerdemo/tokenizer.go`
- Modify: `cmd/tokenizer-demo/main.go`
- Modify: `cmd/recent-chat/main.go`

**Interfaces:**
- Produces: `LoadCounter(tokenizerPath string) (*Counter, error)`
- Preserves: `CountText(text string) (int, []string, []int, error)`

- [x] **Step 1: 写失败测试**

验证不存在的 tokenizer 文件会在 `LoadCounter` 阶段报错，并验证已加载的 tokenizer 能重复执行 `CountText`。

- [x] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/tokenizerdemo`

Expected: FAIL，因为 `LoadCounter` 和保存已加载 tokenizer 的实现还不存在。

- [x] **Step 3: 写最小实现**

让 `LoadCounter` 调用一次 `pretrained.FromFile`，把返回的 tokenizer 保存进 `Counter`；把两个入口改为显式处理初始化错误；增加 `--text` 参数。

- [x] **Step 4: 运行单元测试和两个程序的构建验证**

Run: `go test ./internal/tokenizerdemo ./internal/recentchat`

Expected: PASS。

Run: `go build ./cmd/tokenizer-demo ./cmd/recent-chat`

Expected: exit code 0。

- [x] **Step 5: 运行实践命令**

Run: `go run ./cmd/tokenizer-demo --text 'Tokenizer 不需要我们逐份重写规则。'`

Expected: 输出传入原文、token 数、token 片段和 token IDs。

- [x] **Step 6: review 后提交**

检查初始化错误、兼容性、测试覆盖和是否超出本小节范围，然后提交为一个独立 commit。
