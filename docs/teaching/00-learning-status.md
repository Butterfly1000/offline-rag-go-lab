# Learning Status

主题：当前学到哪里、实现到哪里、下一次从哪里继续

课程定义、依赖和验收以
[Production Offline RAG Course Blueprint](00-course-blueprint.md)
为准；本文件只保存会变化的当前坐标。

## 1. 当前快照

| 状态 | 范围 | 数量 | 说明 |
|---|---|---:|---|
| `已学习` | L01-L23 | 23 | 实践和机器验证完成，用户已确认理解 |
| `已实现待学习` | L24-L38 | 15 | 实践和机器验证完成，用户尚未逐节确认理解 |
| `已规划` | L39-L78 | 40 | 目标、依赖、产物和门禁已确定，尚未实现 |

当前关键坐标：

- 下一次教学：L24。
- 当前实现边界：L38。
- 下一次新实践实现：L39。
- 当前蓝图终点：L78。

按连续主线计算：

- 学习完成度：23 / 78。
- 实践实现度：38 / 78。
- 现有已实现课程中待学习确认：15 节。

## 2. 已学习能力

### L01-L12：Tokenizer、Prompt 与 Token Budget

已经理解并实践：

- Tokenizer 加载、结构检查和 SHA256 身份。
- Ollama context/template 检查与真实模板渲染。
- 完整 conversation token 计数。
- 自动历史预算、严格 recent window 和真实 `/chat`。

阶段结果：

```text
模型容量
  -> 完整计数
  -> 自动预算
  -> 历史裁剪
  -> 生成上限
  -> API 可观测
```

### L13-L18：Session Summary

已经理解并实践：

- 驱逐前缀选择和 token 阈值触发。
- Ollama 滚动摘要与不可信历史边界。
- MySQL version/watermark 乐观锁。
- Summary、recent window 和当前问题共同进入 Chat 预算。

### L19-L23：Long-term Memory

已经理解并实践：

- Memory candidate 校验和真实模型提取。
- 确定性 INSERT、UPDATE、NOOP、FORGET。
- MySQL item/evidence 事务。
- bge-m3、Qdrant 和 `user_id` 隔离检索。

## 3. 已实现待学习

### L24-L28：Dual Retrieval

机器已经验证：

- Memory/Document Hit 与 ownership 边界。
- Document Qdrant scope 过滤和返回后重验。
- 一次 embedding、并行双路检索和故障隔离。
- 独立排序、固定 quota、去重、安全渲染和精确 token 子预算。
- 与 Summary、recent window、Ollama、MySQL 共同接入真实 `/chat`。

真实运行曾观察到 memory=1、document=2、retrieval context=330/512 tokens；不存在 scope
时 document=0；document collection 故障时按策略 warning 降级。

这只证明实现成立，不能替代用户逐节学习确认。

### L29-L33：Production Document Ingestion

机器已经验证：

- 文档、版本、状态机和稳定 chunk identity。
- Markdown heading/fence 与 Go AST 结构化切块。
- MySQL/Qdrant 幂等 ingestion 和稳定 point。
- Snapshot verification、Alias 原子切换和无删除回滚。
- 12 个 Golden Cases、Recall@3/MRR@3、scope isolation 和 forbidden hit。

教学 fixture 的已记录结果是 Recall@3=1、MRR@3=1、scope isolation=100%、
forbidden hit=0。这不代表生产数据已经泛化，也不代表用户已学会。

### L34-L38：Retrieval Quality Engineering

机器已经验证：

- 24 chunks、40 cases、train/validation 固定 checksum 的版本化 Golden Dataset。
- Qdrant named Dense/Sparse、字段感知 BM25 和强制 scope filter。
- 只消费 rank 的 weighted RRF 与逐候选 trace。
- 本地 `qwen:7b` Reranker 严格协议、JSON Schema、deterministic diversity 和 fallback。
- train-only PAVA calibration、81 组 viable grid、按 kind 路由和六类 validation 门禁。
- policy 绑定 dataset/encoder identity，连续真实运行 checksum 一致。

阶段结论不是“所有新组件都默认开启”：L37 Reranker 质量下降且约 30 秒，已明确默认
关闭；L38 policy validation Recall/MRR/NDCG=1、scope isolation=1、forbidden=0，
但 negative pass 仍为 0。实现完成不代表用户已经逐节学会。

## 4. 下一次教学

从 L24 开始，不重讲项目总览，也不直接跳到 L34。

顺序：

```text
L24 Hit 与 ownership
  -> L25 Document Qdrant
  -> L26 Dual Retrieval
  -> L27 合并与预算
  -> L28 真实 Chat
  -> L29-L33 Document Ingestion
```

每节仍按：

```text
问题
  -> 当前实现
  -> 真实运行
  -> 结果解释
  -> 生产差异
  -> 用户确认
```

用户说“懂了”后，只更新对应一节的状态，不整批提前标记。

## 5. 下一次新实践实现

如果继续建设新能力，从 L39 `Evidence and Citation Contract` 开始。

开始 L39 前必须：

1. 读取 [完整蓝图](00-course-blueprint.md) 的阶段七。
2. 读取 [L38 Decision Policy SOP](retrieval-decision-policy-sop.md) 和版本化 Golden Dataset。
3. 保留 L38 的 train/validation 隔离和回归门禁。
4. 先定义 evidence/citation identity 与 refusal 边界，再实现回答生成。

不得直接让模型自由生成 citation，再倒过来寻找来源；source/chunk/version 必须先成为
可验证契约。

教学与新实现可以在不同任务中推进，但状态必须分开：实现完成只进入
`已实现待学习`，不会自动变成 `已学习`。

## 6. 状态更新规则

- 新课程实践和机器验证通过：`已规划 -> 已实现待学习`。
- 用户逐节确认理解：`已实现待学习 -> 已学习`。
- 缺少不可替代外部条件：标记 `阻塞`，同时记录证据和解除条件。
- 只写文档、只提交代码或只跑单元测试，不足以改变阶段完成状态。
- 每五节未来课程还必须通过蓝图中的阶段门禁。

更新本文件时同步检查：

1. [世界地图](../../world/game-world-map.md) 的当前坐标。
2. [完整课程蓝图](00-course-blueprint.md) 的逐课状态。
3. [接手说明](00-handoff-guide.md) 的下一次教学和实现入口。

## 7. 关键证据入口

- Token 与自动预算：[自动预算 Chat SOP](recent-chat-automatic-token-budget-sop.md)
- Session Summary：[Summary Batch Log](00-session-summary-batch-operation-log.md)
- Long-term Memory：[Memory Batch Log](00-long-term-memory-batch-operation-log.md)
- Dual Retrieval：[Dual Retrieval Batch Log](00-dual-retrieval-batch-operation-log.md)
- Document Ingestion：[Document Ingestion Batch Log](00-document-ingestion-batch-operation-log.md)
- Retrieval Quality：[Retrieval Quality Batch Log](00-retrieval-quality-batch-operation-log.md)
- 跨环境问题：[Cross-environment Regression](00-cross-environment-regression.md)

新的优化必须由 Golden Cases、真实失败、安全要求或部署约束驱动。
