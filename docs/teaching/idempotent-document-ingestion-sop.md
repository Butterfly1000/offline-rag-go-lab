# 第 31 节：真实幂等文档入库

本节把第 29 节的版本身份和第 30 节的 chunks 串成真实本地链路：

```text
源文件 -> MySQL version -> tokenizer chunking -> Ollama embedding
      -> Qdrant points -> MySQL manifest ready
```

MySQL、Ollama、Qdrant 都是本机服务，不使用模拟数据或云接口。

## 1. 先实践第一次入库

确认 `config/recent-chat.env` 已配置本地连接后执行：

```bash
go run ./cmd/document-ingest-demo \
  --config config/recent-chat.env \
  --apply-schema \
  --collection offline_rag_document_ingestion_lab_v1 \
  --scope document-ingestion-course \
  --document-id course-markdown \
  --format markdown \
  --source internal/documentingest/testdata/course.md
```

本次真实结果：

```text
Applied schema: sql/document_ingestion.sql
Collection: offline_rag_document_ingestion_lab_v1
Version ID: 1
Noop: false
Chunks: 5
Embed batches: 1
Upsert batches: 1
Manifest rows: 5
```

这证明一次请求真实经过了切分、embedding、Qdrant upsert 和 MySQL manifest 保存。

## 2. 重复执行，观察真正的 no-op

去掉 `--apply-schema`，其他参数保持完全相同：

```bash
go run ./cmd/document-ingest-demo \
  --config config/recent-chat.env \
  --collection offline_rag_document_ingestion_lab_v1 \
  --scope document-ingestion-course \
  --document-id course-markdown \
  --format markdown \
  --source internal/documentingest/testdata/course.md
```

真实结果：

```text
Version ID: 1
Noop: true
Chunks: 5
Embed batches: 0
Upsert batches: 0
Manifest rows: 5
```

这里的幂等不是“用相同 ID 再覆盖一次”。第二次运行根本没有调用 embedding，也没有
调用 Qdrant upsert。MySQL 的 build unique key 找到相同 ready version 后，编排立即返回。

## 3. 检查 Qdrant

```bash
curl -sS http://127.0.0.1:6333/collections/offline_rag_document_ingestion_lab_v1
```

本次真实状态：

```text
status=green
points_count=5
vector size=1024
distance=Cosine
payload indexes=knowledge_scope,document_id,chunk_id
```

`indexed_vectors_count=0` 在只有 5 个 points 时不代表没保存。Qdrant 当前
`indexing_threshold=10000`，小集合使用 full scan；判断是否写入应看 `points_count`。

## 4. MySQL 如何确定“同一个构建”

代码：

- [store_mysql.go](/offline-rag-go-lab/internal/documentingest/store_mysql.go:1)
- [document_ingestion.sql](/offline-rag-go-lab/sql/document_ingestion.sql:1)

逻辑文档唯一键：

```text
(knowledge_scope, document_id)
```

构建唯一键：

```text
(document_source_id, content_hash, parser_version,
 chunk_policy_hash, target_collection)
```

因此以下任意一项变化都会创建新 version：

- 文件规范化内容
- parser 版本
- `max_tokens` 或 `overlap_lines`
- 目标物理 collection

`INSERT ... ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)` 同时处理首次插入和重复查找，
随后读取 status。`ready/active` 才能 no-op；`pending/failed` 使用带预期状态的 UPDATE 抢占：

```sql
UPDATE document_versions
SET status = 'building'
WHERE id = ? AND status IN ('pending', 'failed');
```

只有 `RowsAffected=1` 才代表当前 worker 获得构建权，避免两个 worker 同时构建。

## 5. 编排顺序为什么不能交换

代码：

- [ingest.go](/offline-rag-go-lab/internal/documentingest/ingest.go:1)

关键顺序：

```text
FindOrCreateVersion
-> ready/active 提前 no-op
-> ClaimBuild
-> ChunkDocument
-> 按 BatchSize 调 Ollama /api/embed
-> 验证数量、维度、NaN/Inf
-> EnsureCollection
-> Qdrant UpsertBatch(wait=true)
-> 所有 batches 成功
-> MySQL 事务保存 manifests 并 building -> ready
```

manifest 不能先保存。否则 MySQL 会宣称版本 ready，但 Qdrant 可能只写了一半。

失败时只把 `building` 改成 `failed`，错误最多保存 2048 UTF-8 bytes。调用方 context
即使已经取消，失败清理也使用独立且最多 5 秒的 context，避免记录永久停在 building。

## 6. Qdrant 批量请求是什么

代码：

- [qdrant.go](/offline-rag-go-lab/internal/documentingest/qdrant.go:1)

每批请求等价于：

```http
PUT /collections/offline_rag_document_ingestion_lab_v1/points?wait=true
Content-Type: application/json

{"points":[{"id":"稳定 UUID","vector":[...],"payload":{...}}]}
```

payload 保存 scope、document、chunk、结构路径、原文、content hash 和 embedding model。
MySQL manifest 不保存向量，只保存稳定 point ID 和可核对的 chunk 元数据。

`wait=true` 要求 Qdrant 等待本批更新确认；HTTP 成功后才允许继续保存 ready manifest。
client timeout 为 30 秒，错误 body 最多读取 2048 bytes。

## 7. 为什么 point ID 可以重试

point ID 由下面两项确定性生成：

```text
knowledge_scope + NUL + stable chunk_id
```

相同 chunk 重试会得到相同 UUID。若 worker 在第 2 批失败，第 1 批已经写入的 points
不需要生成新身份；重试会覆盖同一批 IDs，而不是制造重复 points。

## 8. 测试

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

当前测试覆盖：

- ready version 在 embedding 前 no-op
- 按 batch embedding/upsert，最后才保存 manifest
- embedding 失败会标记 failed
- `NaN/Inf` 在 Qdrant 写入前硬失败
- caller cancellation 后仍能使用清理 context
- Qdrant `wait=true`、payload 和空 delete selector 防护
- collection 前缀和 UTF-8 错误长度边界

## 9. 当前边界

本节完成单个物理 collection 内的幂等构建和失败重试，但还没有发布 alias。

生产更新不应在对外服务的旧 snapshot 上边写边删。第 32 节会把完整新版本构建到
另一个物理 collection，验证数量和维度后原子切换 alias；回滚只把 alias 切回旧
collection，不删除任何物理 collection。
