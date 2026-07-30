# Handoff Guide

主题：换模型、设备或任务后，如何继续当前教学与实践

## 1. 最小读取顺序

新模型开始前依次读取：

1. [AI Initialization](../../AI_INITIALIZATION.md)
2. [Game World Map](../../world/game-world-map.md)
3. [Production Offline RAG Course Blueprint](00-course-blueprint.md)
4. [Learning Status](00-learning-status.md)
5. [Teaching Protocol](00-teaching-protocol.md)

读完先复述：

- 当前教学从 L24 开始。
- 当前实现边界是 L38。
- 新实践实现从 L39 开始。
- 当前终点是 L78。
- `已实现待学习` 不能自动当成 `已学习`。

如果复述不一致，先以蓝图和学习状态校正，不要立即改代码或开讲。

## 2. 当前接手事实

| 工作类型 | 起点 | 说明 |
|---|---|---|
| 继续教学 | L24 | L24-L38 已实现和机器验证，待用户逐节确认 |
| 继续新实践实现 | L39 | 先定义 Evidence/Citation identity 和 refusal contract |
| 更新课程方向 | 权威蓝图 | 需要真实数据、故障、安全或部署证据 |

## 3. 继续 L24-L28 教学

额外读取：

1. [Context Hit Boundary](context-hit-boundary-sop.md)
2. [Document Qdrant](document-qdrant-sop.md)
3. [Dual Retrieval](dual-retrieval-sop.md)
4. [Context Merge Budget](context-merge-budget-sop.md)
5. [Recent Chat Dual Retrieval](recent-chat-dual-retrieval-sop.md)
6. [Dual Retrieval Batch Log](00-dual-retrieval-batch-operation-log.md)

教学从真实效果开始，再沿代码解释：

```text
Hit 与 ownership
  -> scope/user 隔离
  -> 一次 embedding 与并行检索
  -> 合并、去重和 token 预算
  -> 真实 /chat
```

不要重复实现 Dual Retrieval，也不要因为 batch log 已通过就代替用户确认。

## 4. 继续 L29-L33 教学

额外读取：

1. [Document Identity](document-identity-version-sop.md)
2. [Structured Chunking](structured-document-chunking-sop.md)
3. [Idempotent Ingestion](idempotent-document-ingestion-sop.md)
4. [Snapshot Alias](document-snapshot-alias-sop.md)
5. [Retrieval Evaluation](document-retrieval-evaluation-sop.md)
6. [Document Ingestion Batch Log](00-document-ingestion-batch-operation-log.md)

必须讲清：

- 逻辑文档、版本、chunk、point 和 alias 是不同身份。
- 幂等重试与全量 snapshot 发布解决的是不同问题。
- 教学 fixture 的 1.0 分数不代表生产泛化。

## 5. 继续 L39 新实践

先读取：

1. [课程蓝图阶段七](00-course-blueprint.md)
2. [L38 Retrieval Decision Policy](retrieval-decision-policy-sop.md)
3. [Retrieval Quality Batch Log](00-retrieval-quality-batch-operation-log.md)
4. [优化 Backlog](00-optimization-backlog.md)

L34-L38 已完成实现和机器验证：

```text
生产 Golden Dataset
  -> Sparse Retrieval
  -> Hybrid Fusion
  -> Reranker 与 Diversity
  -> Calibration、动态 Quota 与回归门禁
```

下一批从 L39 开始进入 Evidence/Citation、Context Packing、拒答和回答质量评估。
不能因为检索 NDCG=1 就假设回答一定有依据；L38 negative pass=0 是下一阶段必须保留
的真实边界。

## 6. 推荐提示词

### 继续教学

```text
请先读取：
1. world/game-world-map.md
2. docs/teaching/00-course-blueprint.md
3. docs/teaching/00-learning-status.md
4. docs/teaching/00-teaching-protocol.md

先复述当前教学、实现和终点坐标，再从 L24 开始。每次只讲一个小段，结合真实运行和
代码边界；我确认“懂了”后，才更新对应一节的学习状态。
```

### 继续新实践

```text
请先读取：
1. world/game-world-map.md
2. docs/teaching/00-course-blueprint.md
3. docs/teaching/00-learning-status.md
4. docs/teaching/retrieval-decision-policy-sop.md
5. docs/teaching/00-optimization-backlog.md

当前实现到 L38。请从 L39 开始，先定义 evidence/citation identity、可反查契约和
无证据拒答门禁；按设计 -> 计划 -> 实现 -> 真实验证 -> SOP -> 审核推进。
```

## 7. 一轮工作结束前

无论教学还是实现，都要检查：

1. 哪些课程状态实际发生了变化。
2. 新的运行证据是否进入对应 SOP 或 operation log。
3. [Learning Status](00-learning-status.md) 是否仍与蓝图一致。
4. [Game World Map](../../world/game-world-map.md) 的当前坐标是否需要更新。
5. 新发现是当前正确性问题、正式课程内容，还是 Backlog 候选。

不要把只存在于聊天上下文中的结论当成已经交接。

## 8. 如何判断接手正确

正确接手的模型会：

- 先读蓝图与状态，再决定动作。
- 知道教学 L24、实现 L38、新实践 L39、终点 L78。
- 区分机器验证和用户学习确认。
- 不重复实现已完成课程。
- 不绕过 Golden Dataset 直接做主观优化。
- 在结束前把事实写回唯一正确的文档。
