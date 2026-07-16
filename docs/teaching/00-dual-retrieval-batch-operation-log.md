# Dual Retrieval Batch Operation And Impact Log

主题：第 24-28 节执行过程、外部状态影响和验证证据。

本日志记录实现行为，不表示用户已经学会对应课程。

## 授权边界

- 只修改 /offline-rag-go-lab
- 每节 RED -> GREEN -> 实践/SOP -> review -> 独立 commit
- 不执行 git push
- 只允许新建和写入 offline_rag_document_chunks_v1
- 不修改 ollama_chat_memory
- memory 查询强制 user_id，document 查询强制 knowledge_scope
- MySQL 是 memory 事实源，Qdrant 失败不能反向修改 MySQL

## 第 24 节：统一 Hit 与 Ownership 边界

### 影响分析

本节只新增纯 Go 类型、校验器、错误分类、单元测试、demo 和文档。

未执行：

- MySQL 连接或写入
- Ollama 请求
- Qdrant 请求或 collection 写入
- 外部网络请求

### RED 证据

命令：

    go test ./internal/contextretrieval -run 'Test(ValidateHit|SourceError)'

结果：FAIL。失败原因是 Hit、ValidateHit、InfrastructureFailure 等目标 API 尚不存在。

### GREEN 证据

命令：

    go test ./internal/contextretrieval
    go test -race ./internal/contextretrieval

结果：PASS。

### 实践行为

命令：

    go run ./cmd/context-hit-demo

目标：展示合法 memory、合法 document 和被拒绝的混合 ownership。

### 当前边界

本节只定义结果边界。真实 Qdrant filter、document collection 和双路召回在后续课程实现。

### Review 发现与修复

Review 发现调用方可以直接构造未知 source 的 SourceError；旧版
IsInfrastructureFailure 只检查 kind，可能把这种畸形错误误判为可降级错误。

修复过程：

1. 新增 TestIsInfrastructureFailureRejectsMalformedSourceError
2. 确认测试先失败
3. 分类时同时检查已知 source、infrastructure kind 和非空 cause
4. 重新运行 package 与全量门禁

## 第 25 节：真实文档 Qdrant 索引与 Scope 检索

### 影响分析

本节新增文档切块身份、Qdrant HTTP 客户端、真实 demo、单元测试与 SOP。

允许的外部状态变化：

- 创建或校验 `offline_rag_document_chunks_v1`
- 为 `knowledge_scope`、`document_id` 创建 keyword payload index
- 用稳定 UUID upsert 三个固定教学点，重跑覆盖相同点
- 调用本地 Ollama `bge-m3` 生成 fixture 与查询向量

明确不执行：

- 不连接或写入 MySQL
- 不修改 `offline_rag_memory_items_v1`
- 不修改 `ollama_chat_memory`
- 不访问外部网络

### RED 证据

命令：

    go test ./internal/contextretrieval -run 'Test(Document|Deterministic)'

结果：FAIL。失败原因是 `DocumentChunk`、`DocumentQdrant` 和稳定 ID API 尚不存在。

### GREEN 证据

命令：

    go test ./internal/contextretrieval

结果：PASS。由于 `httptest.Server` 需要监听本地端口，受限沙箱拒绝 IPv6 bind；
在获准的本机执行环境中重跑后通过。这是执行环境限制，不是 Qdrant 客户端失败。

### 实践行为

先用 curl 只读确认 Ollama 与 Qdrant，再执行：

    go run ./cmd/document-qdrant-demo --config config/recent-chat.env --apply

实际关键输出：

    Embedding model: bge-m3
    Vector dimension: 1024
    Collection: offline_rag_document_chunks_v1 (Cosine)
    Primary top result: Token Budget score=0.708186
    Other scope top result: Another Course score=0.436268
    Cross-scope point present: false
    Idempotent point IDs: true

重复运行后只读检查 collection：

- `points_count=3`，没有因重跑新增重复点
- `vectors.size=1024`
- `vectors.distance=Cosine`
- `knowledge_scope` 和 `document_id` 均为 keyword payload index

### 当前边界

本节使用固定 fixture 讲清真实索引和隔离查询。生产文档解析、切块 worker、
collection alias 和版本重建属于后续优化，不在 demo 中复杂化。

### Review 发现与修复

Review 发现稳定 point ID 只使用 `knowledge_scope + chunk_id`，但原教学注释可能被
理解为 chunk ID 只需在单篇文档内唯一。同一 scope 的两篇文档若都使用
`chunk-001` 会覆盖同一个 Qdrant 点。

修复：明确 chunk ID 必须在 scope 内唯一，并说明生产中可组合 document ID 与稳定
序号/哈希；同时补齐 point ID、空正文、内容哈希、embedding model 和非有限 score
的返回后重验测试。
