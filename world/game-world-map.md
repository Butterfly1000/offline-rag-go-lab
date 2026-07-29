# Game World Map

这是 `offline-rag-go-lab` 的世界入口。

详细课程顺序、依赖和验收以
[Production Offline RAG Course Blueprint](../docs/teaching/00-course-blueprint.md)
为准；当前学到哪里以 [Learning Status](../docs/teaching/00-learning-status.md) 为准。

## 这个世界是什么

`offline-rag-go-lab` 是一个用 Go 逐步建设生产级离线 RAG 的教学与实践仓库。

它已经不只是早期的 mock gateway。当前真实实践已经覆盖：

- Qwen Tokenizer、Prompt Template 和完整上下文预算。
- Ollama 真实对话和滚动 Session Summary。
- MySQL 消息、摘要、Memory Item 和文档版本事实。
- bge-m3 Embedding。
- Qdrant Memory/Document 独立索引与隔离检索。
- Memory + Document 双路检索、合并、Token 预算和真实 `/chat`。
- 文档身份、结构化切块、幂等 Ingestion、Snapshot Alias 和 Retrieval Evaluation。

仓库的目标不是把所有生产技术一次堆进来，而是让每个能力都经历：

```text
理解问题
  -> 实现最小正确边界
  -> 真实运行
  -> 机器验证
  -> 用户确认学习
  -> 进入下一关
```

## 当前坐标

| 坐标 | 范围 | 数量 | 含义 |
|---|---|---:|---|
| 已学习 | L01-L23 | 23 | 实践和机器验证完成，用户已确认理解 |
| 已实现待学习 | L24-L33 | 10 | 实践和机器验证完成，尚待逐节学习确认 |
| 已规划 | L34-L78 | 45 | 目标、依赖、产物和门禁已确定，尚未实现 |

因此：

- 下一次教学从 L24 开始。
- 当前实现边界是 L33。
- 下一次新实践实现从 L34 开始。
- 当前蓝图终点是 L78。

## 十四个区域

| 区域 | 课程 | 状态 | 获得的能力 |
|---|---|---|---|
| 一、Tokenizer 与 Token Budget | L01-L12 | 已学习 | 按真实模型格式计数、裁剪和自动分配 Chat 预算 |
| 二、Session Summary | L13-L18 | 已学习 | 把被驱逐历史滚动压缩并安全接入 Chat |
| 三、Long-term Memory | L19-L23 | 已学习 | 提取、决策、保存、遗忘和隔离检索长期事实 |
| 四、Dual Retrieval | L24-L28 | 已实现待学习 | Memory 与 Document 并行检索、合并和预算 |
| 五、Document Ingestion | L29-L33 | 已实现待学习 | 身份、切块、幂等构建、Alias 发布和评估 |
| 六、检索质量工程 | L34-L38 | 已规划 | Golden Dataset、Sparse/Hybrid、Reranker 和决策策略 |
| 七、有依据的回答 | L39-L43 | 已规划 | Evidence、Citation、拒答和回答质量评估 |
| 八、生产级长期记忆 | L44-L48 | 已规划 | Ontology、异步任务、Outbox、重建和行为评估 |
| 九、生产文档管道 | L49-L53 | 已规划 | 常见格式、OCR、Worker、增量同步和数据血缘 |
| 十、离线模型与资产 | L54-L58 | 已规划 | Model Manifest、离线包、兼容矩阵和容量回滚 |
| 十一、运行时可靠性 | L59-L63 | 已规划 | 幂等、并发、降级、资源治理和 SLO |
| 十二、安全与隐私 | L64-L68 | 已规划 | 权限、攻击防御、审计、导出和可靠删除 |
| 十三、可观测与恢复 | L69-L73 | 已规划 | 日志、指标、健康、备份恢复和事故修复 |
| 十四、交付与最终验收 | L74-L78 | 已规划 | 离线发布、Linux 部署、升级回滚和 Boss 战 |

## 两条继续路线

### 继续学习

从 L24 的真实效果开始：

```text
L24-L28 Dual Retrieval
  -> L29-L33 Document Ingestion
```

代码已经存在，但仍要逐节经历“运行效果、代码边界、生产差异、用户确认”。不能因为实现
完成就自动把这些课程标记为已学习。

### 继续建设

从 L34 开始：

```text
L34 Production Golden Dataset
  -> L35-L38 检索质量工程
  -> L39-L43 有依据的回答
  -> ...
  -> L78 Production Acceptance Boss
```

新的实现必须由蓝图中的前置依赖和验收门禁约束。Backlog 只提供候选和历史背景，不能
绕过蓝图直接决定下一课。

## 当前真实主链路

### 文档进入系统

```text
源文档
  -> 文档/版本身份
  -> 结构化 Chunk
  -> bge-m3 Embedding
  -> Qdrant Snapshot
  -> 逐 Point 验证
  -> Alias 原子切换
  -> Golden Retrieval Evaluation
```

### 一轮 Chat

```text
当前问题
  -> 一次 Embedding
  -> Memory Retrieval + Document Retrieval
  -> Ownership 重验、排序、去重和 Token 子预算
  -> Session Summary + Recent Window
  -> 完整 Prompt Budget
  -> Ollama 生成
  -> MySQL 写入消息
```

### 长期记忆

```text
来源消息
  -> 模型提取 Candidate
  -> Go 严格校验
  -> 确定性 INSERT / UPDATE / NOOP / FORGET
  -> MySQL Item + Evidence
  -> bge-m3 + Qdrant 用户隔离索引
```

## 当前终局

L78 不是“服务能启动”或“演示请求返回 200”。它要求在干净 Linux 单机或小型内网环境
完成：

1. 断网安装和冷启动。
2. 真实语料导入、更新、删除和重建。
3. 检索、引用、回答、记忆和隔离质量回归。
4. 多用户并发对话。
5. Ollama、Qdrant、MySQL 和 worker 故障演练。
6. 备份恢复。
7. 版本升级与失败回滚。
8. 由另一位工程师依据运行手册复现。

当前世界边界不包含公网 SaaS、Kubernetes、多地域、复杂前端和模型训练平台。以后如果
真实目标改变，可以扩展世界，但必须先更新蓝图和验收定义。

## 推荐入口

1. [完整课程蓝图](../docs/teaching/00-course-blueprint.md)
2. [当前学习状态](../docs/teaching/00-learning-status.md)
3. [教学协议](../docs/teaching/00-teaching-protocol.md)
4. [接手说明](../docs/teaching/00-handoff-guide.md)
5. [优化 Backlog](../docs/teaching/00-optimization-backlog.md)

一句话总结：

**这是一个把离线 RAG 从“能跑的真实闭环”逐关推进到“可评估、可隔离、可恢复、可交付”
的 Go 训练场。**
