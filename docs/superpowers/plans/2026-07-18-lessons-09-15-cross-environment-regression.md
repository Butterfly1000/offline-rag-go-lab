# Lessons 09-15 Cross-Environment Regression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为第 9-15 节提供跨机器可执行回归，并把本地资产、Ollama、MySQL 和服务状态问题集中记录为 SOP。

**Architecture:** 按依赖边界拆成三个 shell 脚本：9-10 只依赖 Go 与 Tokenizer；11-12 增加 Ollama，并用显式 `--live` 才访问 `/chat`；13-15 将纯策略、Tokenizer 选择和真实 Ollama 摘要组合。所有脚本复用第 8 节的资产路径约定，不读取或输出数据库密码。

**Tech Stack:** POSIX shell、Go 1.26、Qwen tokenizer.json、Ollama、可选本地 recent-chat/MySQL

## Global Constraints

- 不提交 `config/recent-chat.env`、Tokenizer 模型资产或数据库内容。
- 默认回归不修改 MySQL；只有第 12 节显式 `--live` 会写入独立测试 session。
- 每组完成 review 和验证后独立 commit，不 push。

---

### Task 1: 第 9-10 节本地 Token 回归

**Files:**
- Create: `scripts/regression/lessons-09-10.sh`
- Modify: `docs/teaching/00-cross-environment-regression.md`
- Modify: `docs/teaching/conversation-token-count-sop.md`
- Modify: `docs/teaching/recent-window-template-token-sop.md`

- [x] 验证 Tokenizer 路径、本地 replace、conversation 黄金结果和严格窗口测试。
- [x] 从仓库外目录运行，确认脚本能定位模块。
- [x] Review、测试并提交。

### Task 2: 第 11-12 节 Ollama 与真实 Chat 回归

**Files:**
- Create: `scripts/regression/lessons-11-12.sh`
- Modify: `docs/teaching/00-cross-environment-regression.md`
- Modify: `docs/teaching/automatic-history-budget-sop.md`
- Modify: `docs/teaching/recent-chat-automatic-token-budget-sop.md`

- [x] 验证 Ollama、模型、context limit、预算恒等式和超限失败。
- [x] 用 `--live` 验证真实 `/chat` 自动预算与冲突参数，不在默认模式写 MySQL。
- [x] Review、测试并提交。

### Task 3: 第 13-15 节 Summary 回归

**Files:**
- Create: `scripts/regression/lessons-13-15.sh`
- Modify: `docs/teaching/00-cross-environment-regression.md`
- Modify: `docs/teaching/session-summary-trigger-sop.md`
- Modify: `docs/teaching/session-summary-selection-sop.md`
- Modify: `docs/teaching/session-summary-generation-sop.md`

- [ ] 验证 trigger reason、非法统计、连续前缀、ID 空洞和 Tokenizer 路径。
- [ ] 验证真实 Ollama 摘要调用返回非空结果。
- [ ] Review、完整测试并提交。
