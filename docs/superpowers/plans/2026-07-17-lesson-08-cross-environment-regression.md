# Lesson 08 Cross-Environment Regression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为第 8 节提供一条可重复执行的跨机器回归命令，并集中记录 Tokenizer 兼容性问题的诊断和修复方法。

**Architecture:** Shell 脚本只编排现有 Go 测试和 demo，不复制业务逻辑。教学文档解释脚本每个检查项、已知根因和新机器 SOP，第 8 节文档只保留入口链接。

**Tech Stack:** POSIX shell、Go 1.23 module semantics、Go 1.26 runtime、github.com/sugarme/tokenizer、本地 tokenizer.json

## Global Constraints

- 不访问 MySQL、Qdrant、Ollama 或业务公网接口；Go 间接依赖是否下载由本机 module cache 和 `GOPROXY` 决定。
- 只写仓库内 `.cache/go-build`。
- 当前中文黄金样例必须返回 15 tokens，旧结果 8 视为失败。

---

### Task 1: 第 8 节回归入口与排坑说明

**Files:**
- Create: `scripts/regression/lesson-08.sh`
- Create: `docs/teaching/00-cross-environment-regression.md`
- Modify: `docs/teaching/qwen-message-format-sop.md`

**Interfaces:**
- Consumes: `go.mod` 的本地 replace、`assets/tokenizers/qwen2/tokenizer.json`、现有 Go 测试和 demo
- Produces: `sh scripts/regression/lesson-08.sh`，失败时返回非零状态并指出失败环节

- [x] **Step 1: 编写回归脚本**

脚本依次检查 Go、模块根目录、本地 replace、Tokenizer 资产、相关单元测试、中文黄金计数、合法和非法消息角色。

- [x] **Step 2: 编写排坑文档**

记录 Go 版本判断、模块 replace、正则回溯引用、rune/byte offset、旧 Token 结果、资产路径和构建缓存问题。

- [x] **Step 3: 在第 8 节加入回归入口**

明确先运行统一脚本，再学习消息格式实现。

- [x] **Step 4: 运行验证**

Run: `sh scripts/regression/lesson-08.sh`

Expected: 所有检查显示 `[PASS]`，最后输出 `Lesson 08 cross-environment regression passed.`

- [x] **Step 5: Review 并提交**

检查脚本无外部服务副作用、文档命令可直接运行，然后提交单独 commit。
