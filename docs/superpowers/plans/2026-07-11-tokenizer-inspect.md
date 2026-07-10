# Tokenizer Inspect Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供一个 Go 命令，读取 `tokenizer.json` 并展示决定编码行为的顶层组件类型和词表规模。

**Architecture:** `internal/tokenizerdemo` 增加只负责读取配置摘要的 `InspectFile`；`cmd/tokenizer-inspect` 负责参数和终端输出。检查器只用于教学与兼容性诊断，真实 token 计算仍由 tokenizer 库执行。

**Tech Stack:** Go、标准库 `encoding/json`、Go `flag`、Go `testing`

## Global Constraints

- 不打印完整词表和 added tokens。
- 不自行执行 normalizer、pre-tokenizer 或 BPE 规则。
- 不根据配置摘要断言 tokenizer 与某个模型完全匹配。
- 完成后先 review 和验证，再单独 commit，不 push。

---

### Task 1: Tokenizer 配置摘要检查器

**Files:**
- Create: `internal/tokenizerdemo/inspect.go`
- Create: `internal/tokenizerdemo/inspect_test.go`
- Create: `cmd/tokenizer-inspect/main.go`
- Create: `docs/teaching/tokenizer-inspect-sop.md`

**Interfaces:**
- Produces: `InspectFile(path string) (TokenizerSummary, error)`
- Produces: `TokenizerSummary`，包含 version、六类组件和两类词表数量

- [x] **Step 1: 写失败测试**

用最小 tokenizer JSON 固定组件类型与数量，并固定无效 JSON 返回错误。

- [x] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/tokenizerdemo`

Expected: FAIL，因为 `InspectFile` 和 `TokenizerSummary` 尚不存在。

- [x] **Step 3: 写最小实现和命令入口**

使用 `encoding/json` 读取必要字段；命令默认检查 `assets/tokenizers/qwen2/tokenizer.json`。

- [x] **Step 4: 运行单元测试和真实文件检查**

Run: `go test ./internal/tokenizerdemo`

Expected: PASS。

Run: `go run ./cmd/tokenizer-inspect`

Expected: 输出组件类型、基础词表数量和 added token 数量。

- [x] **Step 5: 补 SOP、review、全量验证并提交**

检查错误处理、输出准确性、测试覆盖和范围，再创建本小节独立 commit。
