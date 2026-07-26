# L24：统一 Hit，不混淆两种 ownership

对应工程课：[Retrieval 小节 24](../context-hit-boundary-sop.md)。本课让 memory 与 document 使用同一 prompt-facing 结构，同时保留各自不可互换的隔离条件。

## 1. 本课一句话

统一 `Hit` 只统一字段形状：memory 必须由 `user_id` 归属，document 必须由 `knowledge_scope` 归属；两种 ownership 不能混在同一 hit。

## 2. 先用人话理解

同一种快递单可以寄个人包裹或知识库文件，但个人包裹必须写收件人，知识库文件必须写许可范围；使用同一纸张不表示可以拿“范围”替代“收件人”。

## 3. 系统里有什么

- 通用字段：`Source`、`ID`、`Content`、`Score`、`Kind`、`Title`、`SourceRef`、`Metadata`。
- memory hit：`Source=memory`、非空 `UserID`、空 `KnowledgeScope`。
- document hit：`Source=document`、非空 `KnowledgeScope`、空 `UserID`。
- `ValidateHit`：裁剪字段、检查 score 有限、验证 source ownership，并复制 metadata。

## 4. 一条完整链路

memory/document adapter 从外部检索结果构造 Hit → `ValidateHit` 根据 Source 执行不同 ownership 规则 → 复制并规范化 Metadata → 上层才允许把 hit 放进后续 merge/prompt。Qdrant 请求 filter 是第一层，返回 Hit 的 owner/scope 重验是第二层；统一结构绝不抹掉来源边界。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/context-hit-demo
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test -race ./internal/contextretrieval
```

- 两条都是纯本地：不需 MySQL/Ollama/Qdrant/配置，不写业务数据，只可能写 build cache，可安全重复。
- demo 展示合法 memory、合法 document，以及 memory 同时携带 scope 被拒绝；race test 验证并发安全测试，不是外部系统压测。

## 6. 结果怎么看

合法 memory 输出带 `user_id=u-001`，合法 document 输出带 `knowledge_scope=offline-rag-course`。若把 scope 加到 memory hit，得到 `memory hit must not carry knowledge_scope`；反过来 document 携带 user_id 也会失败。空 ID/content、NaN/Inf score、未知 source 同样不能进入 prompt。

## 7. 算一遍或走一遍

输入 document hit `{source=document, id=d-1, content=Go, score=0.8, knowledge_scope=course-a}` 合法。若调用方之后修改输入 `Metadata["document_id"]`，已验证 hit 不会跟着变，因为 validator 创建了新 map；若 metadata key 只有空白或两个 key 裁剪后重复，则验证失败。

## 8. 常见误解

“content 和 score 正确就够了”错误，ownership 缺失就是隔离失败。“Qdrant 已 filter，所以无需重验”错误，索引漂移或错误 payload 仍可能发生。“Metadata copy 只是性能细节”错误，map 是引用类型，不复制会使已验证结果被外部改写。

## 9. 当前实现与生产边界

Infrastructure failure（超时、网络、Qdrant 5xx）表示来源暂不可用，上层可记录 warning 后降级；Integrity failure（跨 user/scope、畸形 payload、未知 source）表示隔离契约破坏，必须硬失败。当前不提供完整鉴权：生产还要保证 user/scope 来自可信认证授权，而不是客户端自报。

## 10. 面试怎么说

我用统一 Hit 方便后续处理，但 Source 驱动 ownership：memory 只认 user_id，document 只认 knowledge_scope。外部检索要“请求 filter + 返回重验”，metadata 深拷贝防止校验后变异；基础设施故障可降级，数据完整性故障必须停止。

## 11. 自检题与答案

问：memory hit 可带 knowledge_scope 吗？答：不可以。问：document hit 可带 user_id 吗？答：不可以。问：Qdrant 超时与跨 scope payload 都只给 warning 吗？答：不是，前者可降级，后者硬失败。

## 12. 事实锚点

- 原课：[context-hit-boundary-sop.md](../context-hit-boundary-sop.md)
- Go 组件：`internal/contextretrieval/{types,validate,errors}.go`
- 演示：`cmd/context-hit-demo/main.go`
- 后续：[L25](L25-document-qdrant.md)、[L26](L26-dual-retrieval.md)
