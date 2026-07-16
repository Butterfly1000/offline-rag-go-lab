# 第 25 节：真实文档 Qdrant 索引与 Scope 检索

本节先运行真实效果，再沿代码理解文档切块如何进入 Qdrant，以及查询为什么不会跨知识范围泄漏。

## 1. 本节最终行为

```text
文档切块 -> Ollama /api/embed -> 1024 维向量
         -> Qdrant offline_rag_document_chunks_v1

用户问题 -> Ollama /api/embed -> knowledge_scope filter -> 文档 Hit
```

memory 使用 `user_id` 隔离“这个用户的事实”；document 使用
`knowledge_scope` 隔离“本次允许查询的知识库”。两者含义不同，不能互换。

## 2. SOP：确认配置

确认被 Git 忽略的 `config/recent-chat.env` 包含：

```text
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_EMBED_MODEL=bge-m3
QDRANT_BASE_URL=http://127.0.0.1:6333
QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
```

不要把密码或本机私有配置提交到 Git。可提交的字段示例位于
`config/recent-chat.env.example`。

## 3. SOP：只读确认服务

```bash
curl -sS --max-time 10 http://127.0.0.1:11434/api/tags
curl -sS --max-time 10 http://127.0.0.1:6333/collections
```

第一条确认 Ollama 可访问并已安装 `bge-m3`；第二条确认 Qdrant 可访问。
这两条命令不写数据。

## 4. SOP：运行真实实践

```bash
go run ./cmd/document-qdrant-demo \
  --config config/recent-chat.env \
  --apply
```

`go run` 会先编译 `cmd/document-qdrant-demo` 及其依赖，再执行生成的临时程序。
`--apply` 是写入确认开关；没有它程序直接退出。程序还会校验集合名必须是
`offline_rag_document_chunks_v1`，避免误写 memory 或其他集合。

预期关键输出：

```text
Embedding model: bge-m3
Vector dimension: 1024
Collection: offline_rag_document_chunks_v1 (Cosine)
Cross-scope point present: false
Idempotent point IDs: true
```

## 5. Part 1：文档切块定义

`internal/contextretrieval/document.go` 定义 `DocumentChunk`：

```go
type DocumentChunk struct {
    KnowledgeScope string // 检索隔离范围，例如一套课程或一个租户知识库。
    DocumentID     string // 原始文档稳定 ID。
    ChunkID        string // 当前 knowledge scope 内唯一的稳定切块 ID。
    Title          string // 展示标题，可选。
    SourceRef      string // 原始来源引用，可选。
    Text           string // 真正参与 embedding 和回答的文本。
}
```

`normalizeDocumentChunk` 使用 `strings.TrimSpace` 去除字段两端空白，并拒绝缺少
scope、document ID、chunk ID 或正文的切块。标题和来源可以为空。

## 6. Part 2：幂等 Point ID

Qdrant upsert 需要稳定 point ID，否则每次重跑可能新增重复点。本项目用：

```text
SHA256(knowledge_scope + NUL + chunk_id)
-> 取前 16 字节
-> 设置 UUID version/variant 位
-> 格式化为小写 UUID
```

scope 参与计算，因此两个知识库都叫 `chunk-001` 也不会冲突。同一 scope 和
chunk ID 重跑会得到相同 ID，Qdrant 会覆盖同一个点，这就是本 demo 的幂等性。
反过来，同一 scope 内的两篇文档不能都使用局部 ID `chunk-001`；生产中通常把
`document_id + 分块序号或稳定哈希` 组合成 scope 内唯一的 chunk ID。

## 7. Part 3：建集合与写入

`EnsureCollection` 先 GET 集合。不存在时创建 `Cosine`、1024 维向量集合；已存在
时严格核对维度和距离算法，不一致就报完整性错误，而不是继续写入。

它还为以下 payload 创建 keyword 索引：

- `knowledge_scope`：查询必须使用的隔离条件
- `document_id`：后续按文档删除或重建的定位条件

`Upsert` 写入向量和完整 payload，包括正文 SHA256 `content_hash` 与
`embedding_model`。哈希用于读取后检查 payload 正文是否与写入身份一致。

## 8. Part 4：带 Scope 检索并再次校验

`Search` 请求 `/collections/{name}/points/query`，核心 filter 是：

```json
{"must":[{"key":"knowledge_scope","match":{"value":"offline-rag-course"}}]}
```

Qdrant filter 是第一道隔离。程序收到结果后仍会逐条检查：

- payload 的 `knowledge_scope` 必须等于请求 scope
- point ID 必须能由 payload scope 与 chunk ID 重新计算出来
- `content_hash` 必须匹配正文
- embedding model、正文和身份字段不能缺失
- score 必须是有限数值

这叫“查询过滤 + 返回后重验”。即使索引数据损坏或服务返回异常 payload，也不把
错误文档交给模型。跨 scope 或畸形 payload 属于完整性错误，不能静默降级。

## 9. 当前实现与生产级边界

当前实现已经是真实 Ollama + Qdrant 路径，适合学习和小规模落地；demo 只写三个
固定点。生产环境通常还要增加：

- 独立 ingestion worker：解析文件、切块、批量 embedding、批量 upsert
- collection alias：新版本全量重建后原子切换，避免边服务边清空旧集合
- 文档版本与删除任务：文档更新时清理旧 chunk
- tenant 授权：`knowledge_scope` 必须由服务端授权结果产生，不能直接信任客户端字段
- embedding/model 迁移：模型或维度变化时创建新集合，不在原集合混写

这些是后续优化项，不影响本节要掌握的主框架：稳定身份、真实写入、强制过滤、返回重验。

## 10. 本节重点

1. 文档向量库保存的是“切块 payload + embedding”，不是只保存一段文本。
2. 稳定 point ID 使重跑可幂等，scope 参与 ID 避免跨知识库冲突。
3. `knowledge_scope` filter 必须进入 Qdrant 请求，不能检索完再由 Go 过滤。
4. Qdrant 返回结果仍需重验 ownership、ID、哈希和字段完整性。
5. 基础设施故障可以由上层决定降级；跨 scope 等完整性故障必须硬失败。
