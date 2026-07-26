# L32：快照发布与 alias 回退

对应源课：`docs/teaching/document-snapshot-alias-sop.md`

## 1. 本课一句话

L31 只把一份构建做成 `ready`；L32 先逐项验证这份具体快照，再用一次 Qdrant alias 原子 action 指向它，并在 MySQL 记录 active pointer——两边没有共同事务，所以必须准备 reconciliation，而不是假装“失败已自动回滚”。

## 2. 先用人话理解

把 physical collection 想成两排已验货的货架，alias 是顾客看到的活动指示牌。发布是在同一张 Qdrant 指令里把旧牌摘下、把新牌挂上；MySQL 再把“哪些文档版本 active”写入自己的账本。两件事跨两个系统，不能一起提交。若牌已换而账本写失败，真实状态是“牌已换、账本待对账”，不是“什么也没发生”。

## 3. 系统里有什么

`SnapshotManifest` 包含 scope、目标 collection、ready version、chunk manifest 与 digest。`Publisher.Verify` 检查 digest、向量维度与 Cosine、`knowledge_scope`/`document_id`/`chunk_id` payload index、scope 内准确 point 数，以及每个 point 的 ID、scope、chunk ID、content hash。`ResolveAlias` 只读 `/aliases`；`SwitchAlias` 向 `/collections/aliases` 发一次带 delete/create 两个 action 的请求。MySQL 只在 alias 切换成功后，把快照版本及 `document_sources.active_version_id` 放入单个 MySQL transaction。

## 4. 一条完整链路

L31 产生某 physical collection 的 `ready` manifest → 读取该具体 snapshot → Verify 全部通过且报告未过期 → 验证 MySQL 版本仍 ready/active、alias 仍等于 `--from` → Qdrant 单请求原子换 alias → MySQL activate pointer。MySQL 成功才是发布成功；若它失败，返回 `ReconciliationRequired=true` 和错误，停止后续操作并按实际 alias/数据库状态人工对账。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/documentingest -run 'Test(Publisher|QdrantAlias)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-publish-demo --config /absolute/path/to/your/recent-chat.env --mode resolve
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-publish-demo --config /absolute/path/to/your/recent-chat.env --mode publish --scope document-ingestion-course --collection offline_rag_document_ingestion_lab_v2 --from offline_rag_document_ingestion_lab_v1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-publish-demo --config /absolute/path/to/your/recent-chat.env --mode rollback --scope document-ingestion-course --from offline_rag_document_ingestion_lab_v2 --to offline_rag_document_ingestion_lab_v1
```

- test 使用本地 fake，安全重复，只写 build cache。
- `resolve` 依赖 MySQL/Qdrant 和由 `config/recent-chat.env.example` 派生的私有配置；它只读 alias，可反复运行，用输出确认真实 target。
- `publish` 先读/验证目标 ready snapshot，随后真的改 alias 和 MySQL active pointer；只在 resolve 确认 `--from`、目标 collection 与 manifest 后运行。重复执行通常会因 alias 已不等于 `--from` 失败，这是防止误切换的保护；不要通过盲目重试恢复。
- `rollback` 也真的改 alias 和 MySQL pointer；`--from` 必须是当前 alias target，`--to` 是已验证的旧 collection。它不删除任何 physical collection（删除数为 0）；若 MySQL 阶段失败，先 resolve 并查 MySQL 后做 reconciliation。

## 6. 结果怎么看

`resolve` 只输出 alias 与 target。成功 publish 输出 `Alias switched: true` 和 `MySQL activated: true`；两者缺一不可。若返回“alias switched but MySQL activation requires reconciliation”，alias 已经改变，不能把它称为回滚成功。rollback 成功同样要同时看到 alias 与 MySQL 均已激活，并明确 `Collections deleted: 0`。

## 7. 算一遍或走一遍

假设 alias 原来指向 `..._v1`，`..._v2` 的 manifest 有 40 个 chunks。先检查 digest、40 个 scope points、每个 point 的 identity/hash、1024 Cosine 和三个 payload index。通过后，Qdrant 在一份 actions 请求中删除 alias→v1、创建 alias→v2；再把 v2 versions 写为 active。若第二步成功、最后 MySQL 失败：现在 alias=v2，MySQL 可能仍指 v1，结论只能是 `ReconciliationRequired`，不得臆测 alias 已切回 v1。

## 8. 常见误解

“ready 就可以直接对外用”错误，ready 只是 L31 的构建完成。“两系统失败会自动全回滚”错误，没有跨系统事务。“rollback 会删除坏 collection”错误，它只切回旧 collection 并同步 pointer，物理 collection 保留。“resolve 也会修复状态”错误，它完全只读。

## 9. 当前实现与生产边界

当前发布只接受 `offline_rag_document_ingestion_lab_` 前缀的实验 collection，验证报告默认最多新鲜 5 分钟（demo 设为 1 分钟）。Qdrant alias 的原子性只覆盖 Qdrant 的单次 actions 请求，不覆盖 MySQL。`DOCUMENT_INGEST_ALIAS` 默认是 `offline_rag_document_ingestion_lab_active`，recent-chat 默认仍使用 `offline_rag_document_chunks_v1`；必须显式把 `QDRANT_DOCUMENT_COLLECTION` 改到需要的 target/alias，发布才会影响真实 `/chat`。

## 10. 面试怎么说

我把入库完成和发布分开：先对具体 ready snapshot 验 manifest、索引、point identity，再用 Qdrant 单 action 切 alias，最后在 MySQL transaction 更新 active versions。因为 alias 与 MySQL 无分布式事务，alias 成功而 MySQL 失败时返回 reconciliation-required，保留事实并对账，而不是编造回滚结果。

## 11. 自检题与答案

问：发布前为何既要 count 又要 fetch points？答：count 证明 scope 内数量，fetch 逐个证明 manifest identity/hash 一致。问：alias 成功、MySQL 失败时该做什么？答：停止，把状态标为需对账，resolve alias 并检查 MySQL 后再处理。问：rollback 删除旧 collection 吗？答：不会，删除数为 0。问：为什么不自动进入 chat？答：chat 的 collection 配置独立，必须显式修改。

## 12. 事实锚点

代码锚点：`internal/documentingest/publish.go` 的 Verify/Activate/Rollback；`internal/documentingest/qdrant_alias.go` 的 ResolveAlias/SwitchAlias；`internal/documentingest/store_mysql.go` 的 ActivateSnapshot；`cmd/document-publish-demo/main.go`。前置课是 [L31](L31-idempotent-ingestion.md)，后续用 [L33](L33-retrieval-evaluation.md) 测量检索结果。
