# AI Beginner Course Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `docs/teaching/` 的 42 篇非 `00-*` 课程逐篇建立可快速学习和面试复述的小白教材，并通过教师、学生、正确性审核者闭环。

**Architecture:** 原工程教材保持不动，新教材写入 `docs/teaching/beginner/`。F01-F09
覆盖前置课程，L01-L33 保持连续实现主线；第一批创建的 `00-course-map.md` 冻结
`docs/teaching/*.md` 直系 42 篇来源与 42 个目标的映射，后续只按该 manifest 验证，
不递归扫描 `beginner/` 发现来源。
每批最多五课，教师写作后必须经过学生理解门和正确性审核门，返修后才提交。

**Tech Stack:** Markdown、Go 仓库事实锚点、shell/`rg` 链接与结构检查、`go test ./...`、Git

## Global Constraints

- 不修改现有课程正文和 Go 代码。
- `00-course-map.md` 中冻结的每个 `docs/teaching/*.md` 直系非 `00-*` 原课程恰好
  映射到一个小白课程，`docs/teaching/beginner/**` 永不作为来源。
- 不教读者编写 Go 代码，但必须说明使用的文件、工具、Go 组件、输入、输出和结果。
- 每课必须包含具体算例或状态流转、常见误区、生产边界、30 秒面试复述、自检答案。
- 每个命令必须说明前置依赖、只读或写入副作用、是否可安全重复执行。
- 每个命令必须标为纯本地、外部只读、业务写入或发布/回滚；业务写入和发布命令还要
  写明目标 scope/session/collection/table/alias、幂等性和不能自动恢复的副作用。
- 示例数字必须标明是公式固定值、当前仓库配置值还是某次运行示例值。
- F01-F03 的早期 gateway demo 与 L12/L18/L28 的 recent-chat 服务必须明确区分。
- 每批依次通过教师、AI 小白学生、正确性审核者，重要问题必须返修并重新过两道门。
- 每完成五课提交一次；最后不足五课单独提交。
- L25 必须明确固定 fixture 只验证 scope 检索，L29-L31 必须明确补全真实 ingestion
  生命周期；L25 与 L29 双向链接到同一条
  `fixture 检索 → production-shaped ingestion → snapshot/alias 发布` 关系链。
- L25/L28 默认使用 `offline_rag_document_chunks_v1`；L29-L33 使用
  `offline_rag_document_ingestion_lab_v1/v2` 和
  `offline_rag_document_ingestion_lab_active`。必须明确这是两套 collection 教学链，
  未显式修改 recent-chat 配置时，L29-L33 的结果不会被真实 `/chat` 使用。
- L31 只产生 ready version 与 manifest，不称为已发布 snapshot；失败重试依赖稳定
  point ID 覆盖，当前 ingestion 不自动删除同一物理 collection 的旧 points。
- 教材只展示 `config/recent-chat.env.example` 的占位值，不读取或提交实际
  `config/recent-chat.env` 内容；真实配置仅脱敏核对键是否存在。
- 当前 `cmd/recent-chat` 直接读取配置文件；不能笼统宣称 shell 环境变量总会覆盖配置，
  只有显式支持的回归脚本可使用环境变量。
- 每批提交门禁固定为：结构/数量/链接检查 → `git status --short` →
  只 `git add` 本批明确文件 → `git diff --cached --check` →
  `git diff --cached --name-only` 核对边界 →
  `env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./...` → commit →
  `git status --short` 确认无本批遗漏。
- 只 commit，不 push。

---

### Task 0: 设计基线与开始前验证

**Files:**
- Create: `docs/superpowers/specs/2026-07-26-beginner-course-design.md`
- Create: `docs/superpowers/plans/2026-07-26-beginner-course.md`

**Interfaces:**
- Consumes: 用户目标、42 篇源课盘点、教师/学生/审核者设计门禁
- Produces: 后续九批次共同遵守的范围、模板、事实边界和提交门禁

- [ ] **Step 1: 完成三角色设计审核**

教师确认 42 对 42 范围和模板可执行；学生确认认知顺序、算例和面试验收可理解；审核者
确认事实边界、命令安全和 Git 验证可执行。任何阻塞意见修正后重新审核。

- [ ] **Step 2: 运行开始前基线**

运行：

```bash
git diff --check
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./...
```

空 module cache 的离线机器必须先预置 Go module 依赖；不得把自动联网下载写成课程
保证。

- [ ] **Step 3: 暂存并提交设计基线**

只暂存本 Task 的两个文件，运行 `git diff --cached --check` 和
`git diff --cached --name-only`，确认没有其他文件后提交：
`docs: plan beginner AI course`

### Task 1: 前置课程 F01-F05

**Files:**
- Create: `docs/teaching/beginner/00-course-map.md`
- Create: `docs/teaching/beginner/F01-gateway-chat-pipeline.md`
- Create: `docs/teaching/beginner/F02-gateway-ingest-pipeline.md`
- Create: `docs/teaching/beginner/F03-legacy-chunking.md`
- Create: `docs/teaching/beginner/F04-recent-window-basics.md`
- Create: `docs/teaching/beginner/F05-recent-window-runtime.md`

**Interfaces:**
- Consumes: `01-chat-behavior.md`、`02-ingest-behavior.md`、
  `03-chunking-behavior.md`、`recent-window-layer-01.md`、
  `recent-window-runtime-sop.md`
- Produces: 早期 gateway 与真实 recent-chat 的基础心智模型

- [ ] **Step 1: 冻结 42 对 42 课程 manifest**

先在 `00-course-map.md` 列出 42 条编号、根目录直系原课路径、目标路径、主题和阶段。
后续新增教材不得改变来源集合，只能更新目标链接和三角色验收状态。

- [ ] **Step 2: 教师写五篇初稿**

对照原课、`internal/gateway`、`internal/recentchat`、`cmd/rag-gateway` 和
`cmd/recent-chat`，按设计中的 12 段结构写作。

- [ ] **Step 3: 学生试读并触发返修**

逐课口述输入 → 处理 → 输出、解释真实结果、回答反例和 30 秒面试题；所有不清楚之处
由教师修改后重新试读。

- [ ] **Step 4: 审核者核验并触发复审**

核对服务入口、执行顺序、响应字段、mock/真实依赖边界及文件路径；重要问题修正后由
学生复读、审核者复审。

- [ ] **Step 5: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons F01 to F05`

### Task 2: 前置课程 F06-F09 与 Tokenizer L01

**Files:**
- Create: `docs/teaching/beginner/F06-message-count-distortion.md`
- Create: `docs/teaching/beginner/F07-tokenizer-asset-setup.md`
- Create: `docs/teaching/beginner/F08-content-token-window.md`
- Create: `docs/teaching/beginner/F09-why-session-summary.md`
- Create: `docs/teaching/beginner/L01-tokenizer-load-once.md`

**Interfaces:**
- Consumes: 三篇 recent-window Layer 02 文档、`tokenizer-demo-sop-qwen2.md`、
  `tokenizer-load-once-sop.md`
- Produces: 从消息条数失真到真实 tokenizer 资产和一次加载多次编码的桥梁

- [ ] **Step 1: 教师写五篇初稿**

明确 tokenizer 资产获取、本地 `replace`、加载与编码的区别；算例不能把 token 当字符。

- [ ] **Step 2: 学生试读并触发返修**

学生必须能说清为何需要 `tokenizer.json`、Go tokenizer 库做什么、输出 token IDs/count
代表什么，以及 count window、content-token window、summary 分别解决什么。

- [ ] **Step 3: 审核者核验并触发复审**

核对 `scripts/bootstrap/tokenizer-asset.sh`、`go.mod` replace、`cmd/tokenizer-demo`、
`internal/tokenizerdemo` 和中文回归值的适用边界。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons F06 to L01`

### Task 3: Tokenizer L02-L06

**Files:**
- Create: `docs/teaching/beginner/L02-tokenizer-components.md`
- Create: `docs/teaching/beginner/L03-tokenizer-fingerprint.md`
- Create: `docs/teaching/beginner/L04-ollama-model-metadata.md`
- Create: `docs/teaching/beginner/L05-render-prompt-template.md`
- Create: `docs/teaching/beginner/L06-prompt-token-overhead.md`

**Interfaces:**
- Consumes: L02-L06 对应原 SOP、`tokenizerdemo`、`recentchat/ollama.go`、
  `promptbudget`
- Produces: 从资产结构与身份到真实模型模板和 overhead 的完整解释

- [ ] **Step 1: 教师写五篇初稿**

每课用一个真实命令、一段输出字典和一个边界反例解释组件、SHA256、`/api/show`、
Go template 渲染与 overhead。

- [ ] **Step 2: 学生试读并触发返修**

学生必须能区分结构有效、文件未变、资产与模型匹配这三个不同结论，并能解释
`rendered - content = overhead` 的含义与局限。

- [ ] **Step 3: 审核者核验并触发复审**

核对 JSON 字段、SHA256 行为、Ollama 响应解析、模板渲染入口和示例 token 数来源。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L02 to L06`

### Task 4: Tokenizer 与自动预算 L07-L11

**Files:**
- Create: `docs/teaching/beginner/L07-context-budget.md`
- Create: `docs/teaching/beginner/L08-qwen-message-format.md`
- Create: `docs/teaching/beginner/L09-conversation-token-count.md`
- Create: `docs/teaching/beginner/L10-template-aware-recent-window.md`
- Create: `docs/teaching/beginner/L11-automatic-history-budget.md`

**Interfaces:**
- Consumes: L07-L11 对应原 SOP、`promptbudget`、`chatprompt`、`recentchat`
- Produces: 完整 prompt 计数与自动 history budget 心智模型

- [ ] **Step 1: 教师写五篇初稿**

必须包含 `history = context - fixed - output` 数字算例、Qwen ChatML 边界、完整
conversation 一次编码、从最新向前选择和自动预算失败语义。

- [ ] **Step 2: 学生试读并触发返修**

学生必须能解释 assistant generation prefix、为什么正文 token 求和会少算、超预算为何
必须失败，以及 recent window 如何恢复为正序。

- [ ] **Step 3: 审核者核验并触发复审**

核对预算公式、消息边界字符串、计数调用次数、裁剪顺序、ContextProvider 和
ConversationCounter 失败行为。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L07 to L11`

### Task 5: 自动预算接入与 Summary L12-L16

**Files:**
- Create: `docs/teaching/beginner/L12-chat-automatic-budget.md`
- Create: `docs/teaching/beginner/L13-summary-trigger.md`
- Create: `docs/teaching/beginner/L14-summary-message-selection.md`
- Create: `docs/teaching/beginner/L15-summary-generation.md`
- Create: `docs/teaching/beginner/L16-summary-store.md`

**Interfaces:**
- Consumes: L12-L16 对应原 SOP、`recentchat`、`sessionsummary`、MySQL schema
- Produces: 从 API 自动预算到安全生成并持久化 summary 的前半条链

- [ ] **Step 1: 教师写五篇初稿**

解释三种预算模式、`num_predict`、eviction 前置触发、连续前缀、水位、滚动生成和
version 乐观锁；包含具体 ID 与 token 阈值算例。

- [ ] **Step 2: 学生试读并触发返修**

学生必须能手算 history budget，说明为什么 `evicted=0` 不触发，选出 watermark 与
recent 起点之间的连续消息，并解释生成成功不等于保存成功。

- [ ] **Step 3: 审核者核验并触发复审**

核对 API 字段、阈值逻辑、选择边界、prompt 注入边界、MySQL 首次保存与更新条件。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L12 to L16`

### Task 6: Summary 接入与 Memory L17-L21

**Files:**
- Create: `docs/teaching/beginner/L17-summary-update.md`
- Create: `docs/teaching/beginner/L18-chat-session-summary.md`
- Create: `docs/teaching/beginner/L19-memory-candidate-validation.md`
- Create: `docs/teaching/beginner/L20-memory-extraction.md`
- Create: `docs/teaching/beginner/L21-memory-resolution.md`

**Interfaces:**
- Consumes: L17-L21 对应原 SOP、`sessionsummary`、`recentchat`、`memoryitem`
- Produces: summary 完整编排与不可信 memory candidate 到确定性动作

- [ ] **Step 1: 教师写五篇初稿**

解释 update 顺序、summary reserve、候选可信边界、schema 与 Go validator 的区别，以及
INSERT/UPDATE/NOOP/FORGET 状态流转。

- [ ] **Step 2: 学生试读并触发返修**

学生必须区分 summary 与 memory，能拒绝仅来自 assistant 的事实，并手动判断五种
resolver 场景。

- [ ] **Step 3: 审核者核验并触发复审**

核对失败时状态是否推进、来源消息校验、strict decode、forget 明示条件、等价值与
version 行为。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L17 to L21`

### Task 7: Memory 存储与 Dual Retrieval L22-L26

**Files:**
- Create: `docs/teaching/beginner/L22-memory-store.md`
- Create: `docs/teaching/beginner/L23-memory-qdrant.md`
- Create: `docs/teaching/beginner/L24-context-hit-boundary.md`
- Create: `docs/teaching/beginner/L25-document-qdrant.md`
- Create: `docs/teaching/beginner/L26-dual-retrieval.md`

**Interfaces:**
- Consumes: L22-L26 对应原 SOP、`memoryitem`、`contextretrieval`、MySQL、Ollama、
  Qdrant
- Produces: 带证据的持久记忆、向量检索和双路召回

- [ ] **Step 1: 教师写五篇初稿**

解释 item/evidence 事务、`SELECT FOR UPDATE`、embedding 与生成模型、1024/Cosine、
user/scope ownership、一次 embedding 和并发两路检索。L25 明确固定 fixture 只验证
scope 检索，不负责解析、版本、幂等入库和发布，并链接 L29 的 ingestion 补全课。

- [ ] **Step 2: 学生试读并触发返修**

学生必须能画出不可信候选到 MySQL/Qdrant 的链路，解释不同 user/scope 隔离，并判断
基础设施故障与完整性故障。

- [ ] **Step 3: 审核者核验并触发复审**

核对事务顺序、NOOP evidence、collection 配置、payload index、返回后重验、warning 与
hard failure 规则。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L22 to L26`

### Task 8: Context 合并与 Document Ingestion L27-L31

**Files:**
- Create: `docs/teaching/beginner/L27-context-merge-budget.md`
- Create: `docs/teaching/beginner/L28-chat-dual-retrieval.md`
- Create: `docs/teaching/beginner/L29-document-identity.md`
- Create: `docs/teaching/beginner/L30-structured-chunking.md`
- Create: `docs/teaching/beginner/L31-idempotent-ingestion.md`

**Interfaces:**
- Consumes: L27-L31 对应原 SOP、`contextretrieval`、`recentchat`、
  `documentingest`
- Produces: 安全合并进入 chat，并把真实文档构建成 ready version 与 manifest 的链路

- [ ] **Step 1: 教师写五篇初稿**

解释两路 quota、精确去重、安全转义、完整块 token 预算、三种文档身份、结构化
Markdown/Go chunking、稳定 point ID 和真正 no-op。L28 明确默认 document collection
仍是 `offline_rag_document_chunks_v1`。L29-L31 明确这是把 L25 的“已存在 document
point”前提补全为另一条 ingestion 教学链；L29 双向链接 L25，并重复同一条
`fixture 检索 → production-shaped ingestion → snapshot/alias 发布` 关系链。L31
不得称 ready build 为已发布 snapshot，并说明当前不会自动删除旧 points。

- [ ] **Step 2: 学生试读并触发返修**

学生必须说明 raw score 不能直接混排、prompt 不是权限边界，能手算稳定身份变化，并
从两次入库输出判断是否真的没有重复 embedding/upsert。

- [ ] **Step 3: 审核者核验并触发复审**

核对排序与去重规则、转义方式、token 计数对象、hash 输入、状态机、parser 行为、
manifest 写入时机、ready/active no-op、稳定 ID 重试和旧 point 未自动清理边界；核对
L28 与 L29-L33 的 collection 默认未接通。

- [ ] **Step 4: 验证并提交**

运行 Global Constraints 中的完整批次提交门禁，提交：
`docs: add beginner lessons L27 to L31`

### Task 9: Snapshot、Evaluation 与全课程地图 L32-L33

**Files:**
- Create: `docs/teaching/beginner/L32-snapshot-alias.md`
- Create: `docs/teaching/beginner/L33-retrieval-evaluation.md`
- Modify: `docs/teaching/beginner/00-course-map.md`

**Interfaces:**
- Consumes: L32-L33 对应原 SOP、`documentingest/publish.go`、
  `qdrant_alias.go`、`evaluate.go`、前八批全部教材
- Produces: 发布回滚、检索评估和 42 对 42 的最终阅读入口

- [ ] **Step 1: 教师写两篇课程和课程地图**

L32 从 ready build 开始解释 verified snapshot、原子 alias action、回滚和
reconciliation；同时说明该 alias 默认未被 recent-chat 使用。L33 用具体结果手算
Recall@K/MRR，并说明 scope 越界是门禁失败。课程地图补齐 42 个目标链接和最终三角色
验收状态，但不得改变第一批冻结的来源集合。

- [ ] **Step 2: 学生完成逐课抽查和全链路口述**

除 L32-L33 单课验收外，从每阶段随机抽一课，三分钟讲清身份、输入、token、summary、
memory、retrieval、ingestion、发布和评估的连接。

- [ ] **Step 3: 审核者做全量正确性审核**

检查课程映射、链接、重复编号、事实锚点、公式、环境示例值、demo/生产边界，并复查
前八批遗留的 Minor 问题。

- [ ] **Step 4: 运行最终验证**

运行：

```bash
git diff --check
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./...
```

再从 `00-course-map.md` manifest 运行课程数量、必需标题和本地链接检查；要求其中
冻结的 42 个原课程恰好对应 42 个小白课程，且原课都位于 `docs/teaching/` 直系目录、
目标都位于 `docs/teaching/beginner/`，全部命令退出码为 0。

- [ ] **Step 5: 提交尾批**

按 Global Constraints 的 staged 检查核对尾批文件后，提交：
`docs: complete beginner AI course`

不得执行 `git push`。
