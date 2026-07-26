# AI 小白课程地图（冻结 manifest）

本文件冻结 `docs/teaching/` **直系目录**中 42 篇非 `00-*` 源课与其一对一小白教材目标。`docs/teaching/beginner/` 下的文件永不作为来源。后续批次不得增删或替换来源，只能补齐目标链接与三角色验收状态。

当前批已创建的目标以链接显示；其余目标路径已冻结，待对应批次创建后再转为链接。

| 编号 | 阶段 | 源课 | 小白教材目标 | 主题 |
|---|---|---|---|---|
| F01 | 前置：gateway | `01-chat-behavior.md` | [F01](F01-gateway-chat-pipeline.md) | mock `/chat` 流水线 |
| F02 | 前置：gateway | `02-ingest-behavior.md` | [F02](F02-gateway-ingest-pipeline.md) | mock `/ingest` |
| F03 | 前置：gateway | `03-chunking-behavior.md` | [F03](F03-legacy-chunking.md) | 启发式切块 |
| F04 | 前置：recent-chat | `recent-window-layer-01.md` | [F04](F04-recent-window-basics.md) | 最近消息窗口 |
| F05 | 前置：recent-chat | `recent-window-runtime-sop.md` | [F05](F05-recent-window-runtime.md) | recent-chat 运行验证 |
| F06 | 前置：recent-chat | `recent-window-layer-02-count-distortion.md` | [F06](F06-message-count-distortion.md) | 条数裁剪失真 |
| F07 | 前置：tokenizer | `tokenizer-demo-sop-qwen2.md` | [F07](F07-tokenizer-asset-setup.md) | tokenizer 资产准备 |
| F08 | 前置：recent-chat | `recent-window-layer-02b-token-budget.md` | [F08](F08-content-token-window.md) | content token 窗口 |
| F09 | 前置：summary | `recent-window-layer-02c-session-summary.md` | [F09](F09-why-session-summary.md) | 为什么需要摘要 |
| L01 | Tokenizer | `tokenizer-load-once-sop.md` | [L01](L01-tokenizer-load-once.md) | 加载一次、多次计数 |
| L02 | Tokenizer | `tokenizer-inspect-sop.md` | [L02](L02-tokenizer-components.md) | JSON 组件 |
| L03 | Tokenizer | `tokenizer-fingerprint-sop.md` | [L03](L03-tokenizer-fingerprint.md) | 资产指纹 |
| L04 | Tokenizer | `ollama-model-inspect-sop.md` | [L04](L04-ollama-model-metadata.md) | 模型元数据 |
| L05 | Tokenizer | `prompt-template-render-sop.md` | [L05](L05-render-prompt-template.md) | 模板渲染 |
| L06 | Tokenizer | `prompt-template-token-overhead-sop.md` | [L06](L06-prompt-token-overhead.md) | 模板开销 |
| L07 | Tokenizer | `context-budget-plan-sop.md` | `L07-context-budget.md` | 上下文预算 |
| L08 | Tokenizer | `qwen-message-format-sop.md` | `L08-qwen-message-format.md` | Qwen 消息格式 |
| L09 | Tokenizer | `conversation-token-count-sop.md` | `L09-conversation-token-count.md` | 完整对话计数 |
| L10 | Tokenizer | `recent-window-template-token-sop.md` | `L10-template-aware-recent-window.md` | 模板感知窗口 |
| L11 | Tokenizer | `automatic-history-budget-sop.md` | `L11-automatic-history-budget.md` | 自动历史预算 |
| L12 | Tokenizer | `recent-chat-automatic-token-budget-sop.md` | `L12-chat-automatic-budget.md` | `/chat` 自动预算 |
| L13 | Summary | `session-summary-trigger-sop.md` | `L13-summary-trigger.md` | 触发策略 |
| L14 | Summary | `session-summary-selection-sop.md` | `L14-summary-message-selection.md` | 驱逐消息选择 |
| L15 | Summary | `session-summary-generation-sop.md` | `L15-summary-generation.md` | 滚动摘要生成 |
| L16 | Summary | `session-summary-store-sop.md` | `L16-summary-store.md` | 摘要持久化 |
| L17 | Summary | `session-summary-update-sop.md` | `L17-summary-update.md` | 摘要更新编排 |
| L18 | Summary | `recent-chat-session-summary-sop.md` | `L18-chat-session-summary.md` | 摘要接入 chat |
| L19 | Memory | `memory-item-validation-sop.md` | `L19-memory-candidate-validation.md` | 记忆候选校验 |
| L20 | Memory | `memory-item-extraction-sop.md` | `L20-memory-extraction.md` | 候选提取 |
| L21 | Memory | `memory-item-resolution-sop.md` | `L21-memory-resolution.md` | 生命周期决策 |
| L22 | Memory | `memory-item-store-sop.md` | `L22-memory-store.md` | MySQL 存储 |
| L23 | Memory | `memory-item-qdrant-sop.md` | `L23-memory-qdrant.md` | 向量检索 |
| L24 | Retrieval | `context-hit-boundary-sop.md` | `L24-context-hit-boundary.md` | Hit 与 ownership |
| L25 | Retrieval | `document-qdrant-sop.md` | `L25-document-qdrant.md` | 文档 Qdrant |
| L26 | Retrieval | `dual-retrieval-sop.md` | `L26-dual-retrieval.md` | 双路召回 |
| L27 | Retrieval | `context-merge-budget-sop.md` | `L27-context-merge-budget.md` | 合并与预算 |
| L28 | Retrieval | `recent-chat-dual-retrieval-sop.md` | `L28-chat-dual-retrieval.md` | 接入 chat |
| L29 | Ingestion | `document-identity-version-sop.md` | `L29-document-identity.md` | 身份与版本 |
| L30 | Ingestion | `structured-document-chunking-sop.md` | `L30-structured-chunking.md` | 结构化切块 |
| L31 | Ingestion | `idempotent-document-ingestion-sop.md` | `L31-idempotent-ingestion.md` | 幂等入库 |
| L32 | Ingestion | `document-snapshot-alias-sop.md` | `L32-snapshot-alias.md` | 快照与 alias |
| L33 | Ingestion | `document-retrieval-evaluation-sop.md` | `L33-retrieval-evaluation.md` | 检索评估 |

验收状态：F01–F05、F06–F09、L01–L06 均为“教师完成、学生 APPROVED、正确性审核 APPROVED”；其余课程尚未创建。
