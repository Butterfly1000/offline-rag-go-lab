# L31：把同一 document build 安全地重跑成 ready

对应工程课：[Ingestion 小节 31](../idempotent-document-ingestion-sop.md)。本课将 identity、chunk、embedding、Qdrant 与 manifest 组合成可重试的 build。

## 1. 本课一句话

MySQL 用 build identity 找/建版本；ready/active 同 build 直接 no-op，其他可领取 build 批量 embedding/upsert，所有 batch 成功后最后保存 manifest 并标 ready——ready version 不是 published snapshot。

## 2. 先用人话理解

这是装配线：先给本次构建发唯一工单，已经完成的同一工单直接交付；未完成的才切块、批量制向量、写索引，最后核对清单并标“ready”。“ready”是仓库备货完成，不等于已经把对外货架 alias 切过去。

## 3. 系统里有什么

- build identity：source、content hash、parser version、含 embedding model 的 policy hash、target collection。
- 状态：pending/failed 可 ClaimBuild；ready/active 同 identity 为 no-op。
- batch：chunk → embed → 向量数/维度/有限值校验 → Qdrant upsert。
- manifest：每 chunk 的稳定 ID、ordinal、hash、token、point ID；全部 batch 成功后才写入并转 ready。

## 4. 一条完整链路

FindOrCreateVersion → ready/active 立即 no-op → ClaimBuild（pending/failed→building）→ L30 chunk → 逐 batch embed/校验 → 首 batch EnsureCollection → `UpsertBatch(wait=true)` → 所有 batch 成功 → MySQL transaction 写完整 manifest、building→ready。任一失败标 failed；稳定 point ID 是 `scope + NUL + chunkID`，重试覆盖前次成功 batch 而非重复点。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/documentingest -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-ingest-demo --config /absolute/path/to/your/recent-chat.env --apply-schema --collection offline_rag_document_ingestion_lab_v1 --scope document-ingestion-course --document-id course-markdown --format markdown --source internal/documentingest/testdata/course.md
```

- test 覆盖 ingestion、store_mysql、manifest 与相关身份/状态测试；它不访问真实服务或业务数据，只读取仓库 tokenizer 资产并写 build cache，可安全重复。
- demo 依赖 MySQL、Ollama embedding、Qdrant、tokenizer 与私有配置；`--apply-schema` 写 ingestion 三表，随后读 source、写/复用 version、写 Qdrant collection/points 和 manifest。相同参数的 ready/active build 重跑是 no-op（0 embed/upsert）；failed build 可重新领取。恢复应检查 version 状态/manifest/point IDs，再重跑同 build，不通过删除物理 collection 猜测修复。

## 6. 结果怎么看

首次应有 `Noop: false`、正 chunk/batch/manifest 数；同一 build 第二次有 `Noop: true`、embed/upsert batches 为 0。任何内容、parser/policy（含 embedding model）或 target collection 改变都会是新 build，不应误看作 no-op。

## 7. 算一遍或走一遍

8 个 chunks、batch size 3 会有 3 次 embed/upsert；第 2 批失败时第 1 批 point 已可能存在，但 MySQL 没有 ready manifest，version 标 failed。重跑同 build 用相同 stable point IDs 覆盖第 1 批，再完成所有 batch，最后一次性保存 8 条 manifest 并标 ready。不会因为重试产生 16 个 points。

## 8. 常见误解

“先保存 manifest 更安全”错误，会让未完整写入的向量被声明 ready。“同 collection 的旧 points 会自动删除”错误，当前不自动删同一物理 collection 的旧 points。“ready 就等于 published snapshot”错误，alias 发布是后续步骤。ingestion lab collection 也不会自动进入真实 `/chat`。

## 9. 当前实现与生产边界

已保证：build identity 幂等、ready/active no-op、批量向量校验、稳定 point 重试、manifest 最后 ready。未保证：自动删除同 physical collection 的旧 points、alias 发布或 real/chat 自动切换。此 lab 使用 `offline_rag_document_ingestion_lab_v1/v2`；默认 recent-chat 仍读 `offline_rag_document_chunks_v1`，除非显式改 `QDRANT_DOCUMENT_COLLECTION`。本课只产 ready version+manifest，不称 published snapshot。

## 10. 面试怎么说

我把 ingestion 做成 build 状态机：相同 ready/active build 不做任何 embedding/upsert；可重试 build 用稳定 point ID 覆盖已有 batch，只有全部成功才原子保存 manifest 并标 ready。ready 与 alias published 分离，且 lab collection 不会悄悄影响真实 chat。

## 11. 自检题与答案

问：同 build 已 ready 时还调用 embedding 吗？答：不调用。问：第 2 批失败后能直接写 ready manifest 吗？答：不能。问：会自动删同物理 collection 的旧 points 吗？答：不会。问：ready 是否已发布 snapshot？答：不是。

## 12. 事实锚点

- 原课：[idempotent-document-ingestion-sop.md](../idempotent-document-ingestion-sop.md)
- Go 组件：`internal/documentingest/{ingest,store_mysql,qdrant}.go`
- 演示：`cmd/document-ingest-demo/main.go`
- 前置：[L29](L29-document-identity.md)、[L30](L30-structured-chunking.md)
