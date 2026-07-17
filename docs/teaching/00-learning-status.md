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

**第 1 层 recent window、第 2 层 token/session summary 和第 3 层 long-term memory 都已形成真实闭环，正在进入第 4 层 retrieval 合并。**

也就是说，现在的自然断点是：

- 第 1 层 recent window 完成
- 第 2 层 token budget 与 session summary 完成
- 第 3 层 memory item、MySQL、embedding 与 Qdrant 独立检索完成
- 下一步是 memory retrieval 与 knowledge document retrieval 合并

第 2 层中的 tokenizer 实战已完成：

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

Session Summary 第 13-18 节已经完成真实闭环：

1. 定义 record、watermark 和“驱逐前置 + message/token 阈值”触发策略
2. 按 watermark 与 recent 起点选择连续旧消息前缀，并用真实 tokenizer 计数
3. 用 Ollama 把旧 summary 与新增 evicted 消息生成滚动摘要
4. 用 MySQL version 乐观锁保存 content/watermark，冲突时拒绝覆盖
5. 编排 message source、selector、trigger、generator 和 store
6. 用固定 summary reserve 把 summary + recent + current user 接入自动预算 `/chat`
7. 用真实 MySQL、qwen 和两次 curl 验证 summary 创建、使用、version 和 watermark
8. 真实发现并修复历史消息指令影响弱模型摘要的 prompt-injection 边界

文档：

- [session-summary-trigger-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-trigger-sop.md:1)
- [session-summary-selection-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-selection-sop.md:1)
- [session-summary-generation-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-generation-sop.md:1)
- [session-summary-store-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-store-sop.md:1)
- [session-summary-update-sop.md](/offline-rag-go-lab/docs/teaching/session-summary-update-sop.md:1)
- [recent-chat-session-summary-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-session-summary-sop.md:1)
- [00-session-summary-batch-operation-log.md](/offline-rag-go-lab/docs/teaching/00-session-summary-batch-operation-log.md:1)

Long-term Memory Item 第 19-23 节已经完成独立真实闭环：

1. 定义五类 memory item、candidate、来源消息和 user/session 校验
2. 用真实 `qwen:7b` 提取结构化候选，Go 再做 strict decode 与事实边界校验
3. 用确定性 resolver 决定 INSERT、UPDATE、NOOP、FORGET 和恢复
4. 用 MySQL 事务保存 item 与 evidence，验证 version、rollback 和重复运行
5. 用真实 `bge-m3` 得到 1024 维 embedding
6. 创建独立 1024/Cosine Qdrant collection 和 user/kind payload index
7. 用两个语义相近的测试用户证明 `user_id` 隔离检索
8. 删除 forgotten point，并证明旧 `ollama_chat_memory` 未修改

文档：

- [memory-item-validation-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-validation-sop.md:1)
- [memory-item-extraction-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-extraction-sop.md:1)
- [memory-item-resolution-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-resolution-sop.md:1)
- [memory-item-store-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-store-sop.md:1)
- [memory-item-qdrant-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-qdrant-sop.md:1)
- [00-long-term-memory-batch-operation-log.md](/offline-rag-go-lab/docs/teaching/00-long-term-memory-batch-operation-log.md:1)

### 已实现并验证、尚未确认学会：第 24-28 节 Dual Retrieval

实现状态：

1. 统一 memory/document Hit 与不同 ownership 边界
2. 创建真实 document Qdrant collection，按 `knowledge_scope` 检索并返回后重验
3. 一个 query 只 embedding 一次，并行检索 memory 与 document
4. 基础设施失败可 warning 降级，ownership/畸形数据硬失败
5. 两路独立排序和 quota，memory 优先精确去重
6. 安全渲染 retrieved context，并用真实 tokenizer 精确控制子预算
7. 接入真实 `/chat`，与 recent window、Ollama 和 MySQL 消息写入共同运行

真实验证：memory=1、document=2、retrieval context=330/512 tokens；同 session 后续读取
到两条已写消息；不存在 scope 时 document=0；document collection 故障时 warning 降级。

文档：

- [context-hit-boundary-sop.md](/offline-rag-go-lab/docs/teaching/context-hit-boundary-sop.md:1)
- [document-qdrant-sop.md](/offline-rag-go-lab/docs/teaching/document-qdrant-sop.md:1)
- [dual-retrieval-sop.md](/offline-rag-go-lab/docs/teaching/dual-retrieval-sop.md:1)
- [context-merge-budget-sop.md](/offline-rag-go-lab/docs/teaching/context-merge-budget-sop.md:1)
- [recent-chat-dual-retrieval-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-dual-retrieval-sop.md:1)

注意：以上只代表代码已实现和机器已验证。用户尚未逐节确认“懂了”，不能记录为已学会。

---

## 4. 下一章是什么

下一章应当从这里开始：

### 下一实现章：生产级 Document Ingestion、Chunking 与 Retrieval Evaluation

第 24-28 节的合并框架已经实现。下一步不再重复融合编排，而是补齐文档来源侧：

- 真实文件解析、标题/段落/代码结构识别和稳定 chunk ID
- ingestion worker、批量 embedding、文档版本更新与旧 chunk 删除
- collection alias 全量重建与模型/维度迁移
- 正负查询集、Recall@K、跨 scope 隔离和引用质量评估
- 基于评估再决定 chunk 大小、overlap、reranker、score calibration 和动态 quota

这一章推荐拆成下面几段：

1. 定义生产文档 record、版本、source scope 和稳定 chunk identity（第 29 节已实现验证）
2. 实现 Markdown/代码的结构化 chunking 与可运行 ingestion SOP（第 30 节已实现验证）
3. 实现幂等更新、删除、失败重试和 alias rebuild
4. 建立 retrieval 黄金查询与隔离测试集
5. 用评估结果迭代 chunking、quota 和 reranking

### 已实现并验证、尚未确认学会：第 29 节 Document Identity

实现状态：

1. 定义 `(knowledge_scope, document_id)` 逻辑文档身份
2. 用规范化源文件 SHA256 和 chunk policy hash 确定版本构建
3. 用结构路径、内容 hash 和重复序号生成稳定 chunk ID
4. 证明未变化 chunk 跨版本保持 ID，内容变化、章节移动和重复块会改变 ID
5. 定义 `pending -> building -> ready -> active/failed` 封闭状态机
6. 提供三张 MySQL 表的幂等 schema，但尚未在真实数据库应用

文档：

- [document-identity-version-sop.md](/offline-rag-go-lab/docs/teaching/document-identity-version-sop.md:1)
- [00-document-ingestion-batch-operation-log.md](/offline-rag-go-lab/docs/teaching/00-document-ingestion-batch-operation-log.md:1)

注意：以上只代表代码已实现和机器已验证。用户尚未确认“懂了”，不能记录为已学会。

### 已实现并验证、尚未确认学会：第 30 节 Structured Chunking

实现状态：

1. Markdown 按 ATX heading stack、段落和 fenced code 识别结构
2. Go 使用 `go/parser` 识别 package/import/type/function，并保留 doc comment
3. 所有 chunk 使用真实 tokenizer 强制 `max_tokens`，不使用字符估算
4. oversized 段落按句子/精确前缀切分，代码和声明按完整行及受控 overlap 切分
5. overlap 无法携带新行时不产生重复-only chunk
6. 修复本地 tokenizer fork 的 rune/byte offset、added split 丢失和全局 ID 重排问题
7. 用真实 fixture 证明 Markdown/Go 输出结构、token 数和稳定 chunk ID

文档：

- [structured-document-chunking-sop.md](/offline-rag-go-lab/docs/teaching/structured-document-chunking-sop.md:1)

注意：以上只代表代码已实现和机器已验证。用户尚未确认“懂了”，不能记录为已学会。

下一实现节是第 31 节：真实 MySQL manifest、Ollama batch embedding 和隔离 Qdrant
collection 的幂等 ingestion。

已有概念入口文档：

- [memory-item-qdrant-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-qdrant-sop.md:1)
- [01-chat-behavior.md](/offline-rag-go-lab/docs/teaching/01-chat-behavior.md:1)
- [02-ingest-behavior.md](/offline-rag-go-lab/docs/teaching/02-ingest-behavior.md:1)
- [00-optimization-backlog.md](/offline-rag-go-lab/docs/teaching/00-optimization-backlog.md:1)

---

## 5. 后续推荐学习顺序

建议后续按这个顺序继续：

1. 第 2 层：session summary（已完成真实闭环）
2. 第 3 层：memory item 提取、分类、MySQL、embedding、Qdrant（已完成独立真实闭环）
3. 第 4 层：memory retrieval 和文档 retrieval 如何并存（已实现验证，待用户学习确认）
4. 生产级 chunking / document ingestion 与 retrieval evaluation（下一章）
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
8. [recent-chat-session-summary-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-session-summary-sop.md:1)
9. [00-session-summary-batch-operation-log.md](/offline-rag-go-lab/docs/teaching/00-session-summary-batch-operation-log.md:1)
10. [memory-item-validation-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-validation-sop.md:1)
11. [memory-item-extraction-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-extraction-sop.md:1)
12. [memory-item-resolution-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-resolution-sop.md:1)
13. [memory-item-store-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-store-sop.md:1)
14. [memory-item-qdrant-sop.md](/offline-rag-go-lab/docs/teaching/memory-item-qdrant-sop.md:1)
15. [00-long-term-memory-batch-operation-log.md](/offline-rag-go-lab/docs/teaching/00-long-term-memory-batch-operation-log.md:1)

16. [context-hit-boundary-sop.md](/offline-rag-go-lab/docs/teaching/context-hit-boundary-sop.md:1)
17. [document-qdrant-sop.md](/offline-rag-go-lab/docs/teaching/document-qdrant-sop.md:1)
18. [dual-retrieval-sop.md](/offline-rag-go-lab/docs/teaching/dual-retrieval-sop.md:1)
19. [context-merge-budget-sop.md](/offline-rag-go-lab/docs/teaching/context-merge-budget-sop.md:1)
20. [recent-chat-dual-retrieval-sop.md](/offline-rag-go-lab/docs/teaching/recent-chat-dual-retrieval-sop.md:1)

然后确认第 13-23 节已完成闭环，第 24-28 节已实现验证但尚未由用户确认学会。下一次
教学应从第 24 节真实效果开始逐节讲，不要直接把实现状态改成“已学会”；下一实现章
才是生产级 document ingestion/chunking 与 retrieval evaluation。
