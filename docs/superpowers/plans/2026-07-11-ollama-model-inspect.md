# Ollama Model Inspect Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过 Ollama `/api/show` 读取真实模型的上下文上限和 prompt template。

**Architecture:** 扩展现有 HTTP Ollama client，返回一个只包含教学需要字段的模型摘要；独立命令负责参数和终端输出。模板渲染及精确 token 计算留给下一小节。

**Tech Stack:** Go、Ollama HTTP API、`net/http/httptest`、Go `flag`

## Global Constraints

- 不输出 license、tensor 明细或完整原始响应。
- 不在本节渲染 template。
- 不在本节修改 recent-chat 请求行为。
- 完成后先 review 和验证，再单独 commit，不 push。

---

### Task 1: Ollama 模型摘要

**Files:**
- Modify: `internal/recentchat/ollama.go`
- Create: `internal/recentchat/ollama_show_test.go`
- Create: `cmd/ollama-model-inspect/main.go`
- Create: `docs/teaching/ollama-model-inspect-sop.md`
- Modify: `docs/teaching/00-learning-status.md`

**Interfaces:**
- Add: `HTTPOllamaClient.Show(model string) (OllamaModelSummary, error)`
- Add: `OllamaModelSummary`，包含 model、family、architecture、context length、template 等摘要字段

- [x] **Step 1: 写失败测试并确认 RED**
- [x] **Step 2: 实现 `/api/show` 请求、错误处理和动态 context key 读取**
- [x] **Step 3: 实现命令并运行本机 `qwen:7b` 实践**
- [x] **Step 4: 补 SOP、review、全量验证并提交**
