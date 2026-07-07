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

- [01-chat-behavior.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/01-chat-behavior.md:1)

### 已完成：课程 02 `ingest behavior`

已学会：

- `/ingest` 的核心行为是：
  - 切块
  - 入库
  - 保存原文

文档：

- [02-ingest-behavior.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/02-ingest-behavior.md:1)

### 已完成：课程 03 的部分内容 `chunking behavior`

已学会：

- 当前 demo 怎么识别标题
- 当前 chunker 主要是按“行”切，而不是按段落/语义切

文档：

- [03-chunking-behavior.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/03-chunking-behavior.md:1)

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

- [recent-window-layer-01.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-layer-01.md:1)
- [recent-window-runtime-sop.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-runtime-sop.md:1)

---

## 3. 当前卡位

当前学习位置已经不再是“概念层”，而是：

**第 1 层 recent window 已经真实跑通，正在进入第 2 层：为什么要从按条数裁剪升级到更像生产版的记忆系统。**

也就是说，现在的自然断点是：

- 第 1 层完成
- 第 2 层刚开始

---

## 4. 下一章是什么

下一章应当从这里开始：

### 第 2 层：从“最近”到“重要”

核心问题：

- 为什么 `recent_limit = 1` 时会把真正重要的信息裁掉
- 为什么“最近”不等于“重要”
- 为什么只按消息条数裁剪不够

这一章推荐拆成下面几段：

1. 为什么 count-based recent window 会失真
2. 什么是 token-budget-based recent window
3. 为什么需要 session summary
4. session summary 和 long-term memory 的边界
5. 生产里如何把 recent / summary / memory / retrieval 合并

这一章的第一个正式教学入口文档已经准备好：

- [recent-window-layer-02-count-distortion.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-layer-02-count-distortion.md:1)
- [recent-window-layer-02b-token-budget.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02b-token-budget.md:1)
- [recent-window-layer-02c-session-summary.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02c-session-summary.md:1)

---

## 5. 后续推荐学习顺序

建议后续按这个顺序继续：

1. 第 2 层：token budget + session summary
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

1. [00-teaching-protocol.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/00-teaching-protocol.md:1)
2. [00-learning-status.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/00-learning-status.md:1)
3. [recent-window-layer-01.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-layer-01.md:1)
4. [recent-window-runtime-sop.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-runtime-sop.md:1)
5. [recent-window-layer-02-count-distortion.md](/Users/huangyanyu/offline-rag-go-lab/docs/teaching/recent-window-layer-02-count-distortion.md:1)
6. [recent-window-layer-02b-token-budget.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02b-token-budget.md:1)
7. [recent-window-layer-02c-session-summary.md](/offline-rag-go-lab/docs/teaching/recent-window-layer-02c-session-summary.md:1)

然后从“第 2 层第一小段”开始继续，而不是重新回到项目介绍。
