# 第 32 节：Snapshot 验证、Alias 切换与回滚

本节解决生产发布问题：新文档不能直接覆盖线上索引，而应完整构建到新物理 collection，验证后原子切换稳定 alias。

## 1. 实践结果

本地完整构建：

- v1：Markdown 5 chunks + Go 5 chunks，共 10 points
- v2：更新 Markdown 6 chunks + Go 5 chunks，共 11 points
- vector：1024/Cosine
- alias：`offline_rag_document_ingestion_lab_active`

发布 v2：

```bash
go run ./cmd/document-publish-demo --config config/recent-chat.env \
  --mode publish --scope document-ingestion-course \
  --collection offline_rag_document_ingestion_lab_v2 \
  --from offline_rag_document_ingestion_lab_v1
```

真实输出包括：`Points: 11`、`Alias switched: true`、`MySQL activated: true`。

回滚与恢复：

```bash
go run ./cmd/document-publish-demo --config config/recent-chat.env \
  --mode rollback --from offline_rag_document_ingestion_lab_v2 \
  --to offline_rag_document_ingestion_lab_v1
```

回滚输出 `Collections deleted: 0`。随后再次 publish v2，最终 alias 指向 v2；v1、v2 都仍存在。

## 2. 发布前具体验证什么

入口：[publish.go](/offline-rag-go-lab/internal/documentingest/publish.go:1)

`Verify` 依次检查：

1. collection 只能使用教学隔离前缀
2. vector size 和 Cosine
3. `knowledge_scope`、`document_id`、`chunk_id` payload indexes
4. scope 精确 point count 等于聚合 MySQL manifest 数
5. 按 manifest point IDs 批量取回所有 points
6. 每个 point 的 ID、scope、chunk ID、content hash 与 MySQL 一致

manifest digest 按稳定 point ID 排序后计算。不能只按 chunk ordinal，因为每个文档的 ordinal 都从 0 开始。

## 3. Alias 为什么是原子请求

代码：[qdrant_alias.go](/offline-rag-go-lab/internal/documentingest/qdrant_alias.go:1)

切换只发送一次：

```http
POST /collections/aliases

{"actions":[
  {"delete_alias":{"alias_name":"offline_rag_document_ingestion_lab_active"}},
  {"create_alias":{"alias_name":"offline_rag_document_ingestion_lab_active","collection_name":"..._v2"}}
]}
```

delete/create 是同一 Qdrant action 请求。代码没有 collection DELETE API；回滚也是相同 alias action，只交换 from/to。

## 4. TOCTOU 与双写边界

verification report 绑定 collection、scope、manifest digest、point count 和时间，默认 5 分钟过期。切换前再次解析 alias，当前目标必须等于 `from`，否则拒绝执行。

Qdrant alias 和 MySQL 不存在跨系统事务。顺序是先切 alias，再更新 MySQL active version；若 MySQL 失败，结果明确返回 `ReconciliationRequired=true`，不能谎称整体回滚。

## 5. 两个真实坑

1. 当前 Qdrant 使用 `GET /aliases` 返回 `result.aliases`；`GET /aliases/{name}` 是空 body 404。已用真实响应回归锁定。
2. MySQL 默认 `RowsAffected` 是实际变化行数。active version 已等于目标时 UPDATE 返回 0；代码会再读取当前值，相同才视为幂等成功。

## 6. 测试与边界

```bash
go test ./internal/documentingest -run 'Test(Publisher|QdrantAlias)'
go test -race ./internal/documentingest
```

本节不删除旧 collection。下一节只通过稳定 alias 做检索评估，并用 Golden Cases 检查 Recall@3、MRR@3 和 scope 隔离。
