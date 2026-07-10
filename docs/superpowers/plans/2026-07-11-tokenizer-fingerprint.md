# Tokenizer Fingerprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 tokenizer 文件建立 SHA256 身份基线，并能在启动前发现资产被替换。

**Architecture:** `InspectFile` 在读取配置时同时计算 SHA256；独立校验函数比较实际值与预期值；`tokenizer-inspect` 通过可选参数触发校验。模型来源对照等深入优化统一进入教学优化 backlog。

**Tech Stack:** Go、标准库 `crypto/sha256`、Go `flag`、Go `testing`

## Global Constraints

- SHA256 只证明文件一致性，不证明文件与模型匹配。
- 不引入资产下载、远程查询或复杂 manifest 系统。
- 深入优化记录到 `docs/teaching/00-optimization-backlog.md`。
- 完成后先 review 和验证，再单独 commit，不 push。

---

### Task 1: Tokenizer 文件指纹

**Files:**
- Modify: `internal/tokenizerdemo/inspect.go`
- Modify: `internal/tokenizerdemo/inspect_test.go`
- Modify: `cmd/tokenizer-inspect/main.go`
- Create: `docs/teaching/tokenizer-fingerprint-sop.md`
- Create: `docs/teaching/00-optimization-backlog.md`
- Modify: `docs/teaching/00-teaching-protocol.md`

**Interfaces:**
- Add: `TokenizerSummary.SHA256 string`
- Add: `VerifySHA256(actual string, expected string) error`

- [x] **Step 1: 写失败测试并确认 RED**
- [x] **Step 2: 实现 SHA256 摘要和一致性校验**
- [x] **Step 3: 增加 `--expect-sha256` 并运行真实成功/失败命令**
- [x] **Step 4: 补 SOP、优化 backlog 和教学协议**
- [x] **Step 5: review、全量验证并提交**
