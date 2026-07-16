# 第 26 节：一次 Embedding，并行检索 Memory 与 Document

本节先运行真实效果，再理解为什么同一个问题只生成一次查询向量、两路检索如何并发，以及哪些错误可以降级。

## 1. 本节最终行为

```text
用户问题
  -> Ollama /api/embed（只调用一次）
  -> 同一个 []float32
       |-> memory Qdrant：user_id filter
       |-> document Qdrant：knowledge_scope filter
  -> 分别返回 MemoryHits、DocumentHits、Warnings
```

本节还不把两路结果混成最终 prompt。确定性合并、去重和 token 预算是第 27 节。

## 2. SOP：前置条件

`config/recent-chat.env` 至少包含：

```text
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_EMBED_MODEL=bge-m3
QDRANT_BASE_URL=http://127.0.0.1:6333
QDRANT_MEMORY_COLLECTION=offline_rag_memory_items_v1
QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
```

先完成第 23 节 memory Qdrant fixture 和第 25 节 document fixture。可只读确认：

```bash
curl -sS --max-time 10 http://127.0.0.1:6333/collections/offline_rag_memory_items_v1
curl -sS --max-time 10 http://127.0.0.1:6333/collections/offline_rag_document_chunks_v1
```

## 3. SOP：运行真实双路检索

```bash
go run ./cmd/dual-retrieval-demo --config config/recent-chat.env
```

这个命令只调用 Ollama embedding 和 Qdrant 查询，不写 MySQL、Qdrant。它拒绝任何
非预期集合名，避免把教学查询指向错误集合。

预期关键输出：

```text
Query embeddings: 1
Memory hits:
Document hits:
Retrieval warnings: 0
Cross-user memory present: false
Cross-scope document present: false
```

standalone demo 要求两路服务都健康，只要有 warning 就退出非零；真实聊天服务可以
按本节错误策略带 warning 继续回答。

## 4. Part 1：Memory Adapter

现有 `memoryitem.QdrantIndexer.Search` 返回 `SearchResult`，文档检索返回统一 `Hit`。
`internal/contextretrieval/memory_adapter.go` 是适配层：

```go
type MemoryVectorSearcher interface {
    Search(ctx context.Context, userID string, kind memoryitem.Kind,
        vector []float32, limit int) ([]memoryitem.SearchResult, error)
}
```

它把 memory 结果转换成：

```text
ID:       memory:{item_id}
Content:  {kind}/{key}: {value}
UserID:   请求的 user_id
Metadata: key、version
```

adapter 不盲信底层：返回的 `user_id` 必须仍等于请求用户，ID/version 必须为正，
kind/key/value 不能缺失，score 必须有限。然后再次调用统一 `ValidateHit`。

## 5. Part 2：区分服务故障与数据故障

memory Qdrant 客户端新增 `QdrantDataError`。以下情况说明返回数据不能信任：

- JSON 响应无法解码
- point ID 与 payload memory ID 不一致
- payload 绕过 `user_id` 或 kind filter
- kind、key、value、version、score 畸形

adapter 把它转成 `IntegrityFailure(SourceMemory, err)`，必须硬失败。

连接拒绝、HTTP 500、timeout、context cancel 等没有证明数据越权，adapter 将其转成
`InfrastructureFailure`。上层可以暂时不用该来源，并记录 warning。

## 6. Part 3：为什么只 Embedding 一次

memory 与 document 都使用 bge-m3、1024 维和 Cosine。用户问题的语义向量与数据源
无关，因此 `DualRetriever` 先执行一次：

```go
vectors, err := embedder.Embed(ctx, embeddingModel, []string{req.Query})
vector := vectors[0]
```

再把同一个只读 vector 传给两路 Search。相比各自 embedding，减少一次模型调用、
延迟和资源占用，也避免两路因模型配置不同而不可比较。

## 7. Part 4：两路 I/O 并发

两次 Qdrant 查询互不依赖，顺序调用的总耗时接近 `memory耗时 + document耗时`；并发
调用接近两者较慢值。实现为每个启用来源启动一个 goroutine，并向容量为来源数的
channel 写一个结果：

```go
go func() {
    hits, err := memory.Search(ctx, req.UserID, vector, req.MemoryLimit)
    results <- sourceResult{source: SourceMemory, hits: hits, err: err}
}()
```

主 goroutine 必须收齐所有已启动结果才返回。各 worker 不追加共享 slice，race 测试
验证无数据竞争。收齐后固定按 memory、document 顺序处理，因此 warning 顺序稳定。

## 8. Part 5：故障隔离规则

```go
if IsInfrastructureFailure(err) {
    result.Warnings = append(result.Warnings, err.Error())
    continue
}
return DualResult{}, err
```

- memory timeout、document HTTP 500：保留另一来源结果并返回 warning
- 两路都基础设施失败：没有召回结果，返回两个 warning，不伪造上下文
- embedding 失败：两路都不能查询，不调用 Search，返回一个 warning
- 跨 user、跨 scope、畸形 payload：完整性错误，整个检索硬失败

基础设施降级保证可用性；完整性硬失败保证隔离和数据可信度。

## 9. 当前实现与生产级边界

当前实现已覆盖真实一次 embedding、并发双 Qdrant 查询、结果重验和故障隔离。生产中
通常还会增加每来源 timeout、metrics/tracing、限流、熔断和 warning 结构化错误码；
这些优化不改变本节框架。

尤其要注意：`user_id` 与 `knowledge_scope` 都必须来自服务端认证/授权上下文，不能
直接信任客户端声称的身份或 scope。

## 10. 本节重点

1. 查询向量属于“问题”，同模型同空间下可由两路检索共享。
2. 两个独立远程 I/O 应并发，但必须等待所有已启动 goroutine 结束。
3. memory 和 document 分别保留 user/scope ownership，不因统一 Hit 而混淆。
4. 基础设施失败可降级，数据完整性或越权失败不可降级。
5. 第 26 节只召回并分类；第 27 节才决定最终顺序和 token 占用。
