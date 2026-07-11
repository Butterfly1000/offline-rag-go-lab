# Learning Status

主题：当前学习进度、下一章和最终目标

这份文档回答三个问题：

1. 现在已经学到哪里
2. 下一章应该学什么
3. 整个学习路径最终要走到哪里

---

## 1. 当前总目标

当前学习不是单纯阅读仓库，而是要逐步达到：

1. 读懂当前项目结构和真实行为
2. 跑通项目中的关键链路
3. 理解这些链路和生产级实现的差异
4. 在此基础上继续真实落地

可以压成一句话：

**先理解，再验证，再升级，再实现。**

---

## 2. 当前已完成内容

### 已完成：项目整体理解

已经完成对仓库整体定位的理解：

- 这是一个以 Go 版离线 RAG 为主线的实验项目
- 当前主线是最小闭环，不是完整生产版
- 仓库里还有独立整理的 `qiniu-uploader` 子项目

### 已完成：课程 01 `chat behavior`

已学会：

- `App` 是主编排器
- `/chat` 主流程是：
  - 校验
  - 检索
  - 压缩
  - 生成
  - 写日志

文档：

- [01-chat-behavior.md](/offline-rag-go-lab/docs/teaching/01-chat-behavior.md:1)

### 已完成：课程 02 `ingest behavior`

已学会：

- `/ingest` 的核心行为是：
  - 切块
  - 入库
  - 保存原文

文档：

- [02-ingest-behavior.md](/offline-rag-go-lab/docs/teaching/02-ingest-behavior.md:1)

### 已完成：课程 03 的部分内容 `chunking behavior`

已学会：

- 当前 demo 怎么识别标题
- 当前 chunker 主要是按“行”切，而不是按段落/语义切

文档：

- [03-chunking-behavior.md](/offline-rag-go-lab/docs/teaching/03-chunking-behavior.md:1)

### 已完成：Recent Window Layer 01

这是当前最重要的实战成果。

已学会并已真实验证：

1. `recent-chat` 已真实落地
2. 第一轮无历史时：
   - `used_messages = 0`
   - `recent_window = []`
3. 第一轮结束后：
   - user/assistant 两条消息写入 MySQL
4. 第二轮同 `session_id` 时：
   - 能从 MySQL 读出最近消息
   - `used_messages > 0`
   - `recent_window` 非空
5. `recent_limit = 1` 时：
   - 最近窗口被真实裁剪成 1 条

文档：

- [recent-window-layer-01.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-01.md:1)
- [recent-window-runtime-sop.md](/offline-rag-go-lab/docs/teaching/recent-window-runtime-sop.md:1)

---

## 3. 当前卡位

当前学习位置已经不再是“概念层”，而是：

**第 1 层 recent window 已经真实跑通，正在进入第 2 层：为什么要从按条数裁剪升级到更像生产版的记忆系统。**

也就是说，现在的自然断点是：

- 第 1 层完成
- 第 2 层刚开始

当前在第 2 层中插入 tokenizer 实战小课，已完成：

1. tokenizer 启动时加载一次并重复编码
2. 查看 `tokenizer.json` 的组件结构和词表规模
3. 用 SHA256 固定 tokenizer 文件身份并支持启动前校验
4. 从 Ollama `/api/show` 读取模型上下文上限和 prompt template
5. 用 Go `text/template` 渲染当前 Ollama 模型的 system/user prompt
6. 对比正文 token、rendered prompt token 和 template overhead
7. 用 context limit、固定输入和 output reserve 计算 recent history budget
8. 把单条历史格式化为包含 role 和消息边界的 Qwen ChatML
9. 对完整 system/history/user/assistant-prefix conversation 做一次性 token 计数
10. 让 recent window 按格式化消息 token 严格裁剪
11. 从 Ollama context length、固定输入和回答预留自动计算历史预算
12. 把自动预算、严格裁剪和 `num_predict` 接入真实 `/chat`，并返回预算明细

文档：

- [tokenizer-load-once-sop.md](/offline-rag-go-lab/docs/teaching/tokenizer-load-once-sop.md:1)
- [tokenizer-inspect-sop.md](/offline-rag-go-lab/docs/teaching/tokenizer-inspect-sop.md:1)
- [tokenizer-fingerprint-sop.md](/offline-rag-go-lab/docs/teaching/tokenizer-fingerprint-sop.md:1)
- [ollama-model-inspect-sop.md](/offline-rag-go-lab/docs/teaching/ollama-model-inspect-sop.md:1)
- [prompt-template-render-sop.md](/offline-rag-go-lab/docs/teaching/prompt-template-render-sop.md:1)
- [prompt-template-token-overhead-sop.md](/offline-rag-go-lab/docs/teaching/prompt-template-token-overhead-sop.md:1)
- [context-budget-plan-sop.md](/offline-rag-go-lab/docs/teaching/context-budget-plan-sop.md:1)
- [qwen-message-format-sop.md](/offline-rag-go-lab/docs/teaching/qwen-message-format-sop.md:1)
- [conversation-token-count-sop.md](/offline-rag-go-lab/docs/teaching/conversation-token-count-sop.md:1)
- [recent-window-template-token-sop.md](/offline-rag-go-lab/docs/teaching/recent-window-template-token-sop.md:1)
- [automatic-history-budget-sop.md](/offline-rag-go-lab/docs/teaching/automatic-history-budget-sop.md:1)
- [recent-chat-automatic-token-budget-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-automatic-token-budget-sop.md:1)
- [00-optimization-backlog.md](/offline-rag-go-lab/docs/teaching/00-optimization-backlog.md:1)

token 主线当前状态：

**已形成“模型容量 -> 完整计数 -> 自动预算 -> 历史裁剪 -> 生成上限 -> API 可观测”的真实闭环。**

Session Summary 当前已完成第 13 节：

1. 定义 summary record 和 `last_message_id` 水位
2. 提供 `session_summaries` MySQL 建表 SQL
3. 实现“驱逐前置 + message/token 双阈值”触发策略
4. 用命令和测试验证触发原因及非法输入

文档：

- [session-summary-trigger-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-trigger-sop.md:1)

---

## 4. 下一章是什么

下一章应当从这里开始：

### 第 2 层下一部分：Session Summary

核心问题：

- 历史即使按 token 正确裁剪，旧信息仍然会离开 recent window
- session summary 如何把被裁掉的对话压成持续上下文
- 什么时候触发摘要、摘要存哪里、下一轮如何合并

这一章推荐拆成下面几段：

1. 设计 session summary 数据结构和触发条件（已完成）
2. 选择水位之后且已被驱逐的消息，用 Ollama 真实生成增量摘要
3. 把摘要存入 MySQL memory/session summary store
4. 下一轮组合 summary + recent window + current user
5. 再区分 session summary 和 long-term memory

已有概念入口文档：

- [recent-window-layer-02-count-distortion.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02-count-distortion.md:1)
- [recent-window-layer-02b-token-budget.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02b-token-budget.md:1)
- [recent-window-layer-02c-session-summary.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02c-session-summary.md:1)

---

## 5. 后续推荐学习顺序

建议后续按这个顺序继续：

1. 第 2 层：session summary（token budget 已完成）
2. 第 3 层：memory item 提取、分类、去重、存储
3. 第 4 层：memory retrieval 和文档 retrieval 如何并存
4. 再回头补 chunking / retrieval 的生产级升级
5. 最后做真正的升级实现

---

## 6. 最终落地方向

最终目标不是只保留教学文档，而是把项目逐步推进到更真实的 memory system：

### 第 1 阶段

- recent window
- MySQL message store
- Ollama 对话

### 第 2 阶段

- token-based window
- session summary

### 第 3 阶段

- memory items
- memory store
- selective extraction

### 第 4 阶段

- memory retrieval
- 与知识检索合并
- 更接近生产级上下文管理

---

## 7. 新模型接手时应该先知道什么

新模型继续教学前，应先读取：

1. [00-teaching-protocol.md](/offline-rag-go-lab/docs/teaching/00-teaching-protocol.md:1)
2. [00-learning-status.md](/offline-rag-go-lab/docs/teaching/00-learning-status.md:1)
3. [recent-window-layer-01.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-01.md:1)
4. [recent-window-runtime-sop.md](/offline-rag-go-lab/docs/teaching/recent-window-runtime-sop.md:1)
5. [recent-window-layer-02-count-distortion.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02-count-distortion.md:1)
6. [recent-window-layer-02b-token-budget.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02b-token-budget.md:1)
7. [recent-window-layer-02c-session-summary.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02c-session-summary.md:1)

然后从 [session-summary-trigger-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-trigger-sop.md:1) 复核当前接口，再开始“增量摘要输入选择与 Ollama 生成”，不要重新讲 token 或 summary 概念。
