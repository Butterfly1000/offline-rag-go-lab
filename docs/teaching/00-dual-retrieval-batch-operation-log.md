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

## 第 26 节：共享 Embedding、并行双路召回与故障隔离

### 影响分析

本节新增 memory adapter、memory Qdrant 数据错误类型、并行双路检索、测试、只读
真实 demo 和 SOP。

运行时只读行为：

- 调用本地 Ollama `bge-m3` 一次生成查询向量
- 查询 `offline_rag_memory_items_v1`
- 查询 `offline_rag_document_chunks_v1`

明确不执行：

- 不写 MySQL
- 不创建、upsert 或删除 Qdrant 点
- 不修改 `ollama_chat_memory`
- 不访问外部网络

### RED 证据

命令：

    go test ./internal/contextretrieval ./internal/memoryitem -run 'Test(Memory|Dual|QdrantData)'

结果：FAIL。失败原因是 `NewMemoryQdrantSearcher`、`DualRetriever`、
`QdrantDataError` 等目标 API 尚不存在。补充非有限 score 边界时也先确认点转换 API
不存在，再提取实现。

### GREEN 证据

命令：

    go test ./internal/contextretrieval ./internal/memoryitem
    go test -race ./internal/contextretrieval ./internal/memoryitem

结果：PASS。并发 overlap 测试与两个包 race 测试均通过。

### 调试记录

一次组合测试执行单元未及时回收。使用 `go test -timeout 5s -v ... -run TestDual`
定位后，所有 Dual 测试在 0.277 秒内完成，排除 goroutine 死锁；拆分命令后两个包
分别在约 0.2-0.5 秒完成。问题属于工具执行单元回收，不是代码并发阻塞。

测试还发现最初把 `bad key` 当作非法 memory key，但项目规则会将空格规范化为
下划线。根据真实领域规则改用包含 `/` 的非法 key，没有修改既有业务行为。

### 实践行为

命令：

    go run ./cmd/dual-retrieval-demo --config config/recent-chat.env

实际输出：

    Query embeddings: 1
    Memory hits:
    - memory:1 score=0.577871 project_fact/implementation_language: Go
    Document hits:
    - document:56e3e0f4-1383-45ea-aa3d-c5566356ecad score=0.658747 Token Budget
    - document:3de194f1-af50-4077-bf6e-70abc9183614 score=0.509845 Recent Window
    Retrieval warnings: 0
    Cross-user memory present: false
    Cross-scope document present: false

实际运行未写数据库或向量点；只读取两个固定集合并调用一次本地 embedding。

### Review 发现与修复

Review 确认 adapter 和 document client 已检查 ownership，但 orchestrator 仍应保持自己
的边界，防止未来替换 Searcher 实现后绕过校验。

修复：新增 Searcher 无报错却返回其他 user Hit 的测试，确认 `DualRetriever` 会再次
校验来源与请求 ownership，并返回不可降级的 IntegrityFailure。

独立 review 代理连续超时且没有返回审查结果，已关闭；本地逐项 review 覆盖了错误
分类、goroutine 收敛、固定处理顺序、ownership 重验和 demo 集合 guard。

最终 `go vet ./...` 发现测试使用了 Go 1.24 新增的 `testing.T.Context()`，而模块声明
兼容 Go 1.23。修复为 `context.Background()`，保持项目模块版本不变；重新 vet 通过。

## 第 27 节：确定性合并、安全渲染与精确 Token 预算

### 影响分析

本节新增纯 Go 合并、渲染、预算代码、单元测试、真实 tokenizer demo、SOP，并更新
优化 backlog。

运行时只读取 `RECENT_CHAT_TOKENIZER_PATH` 指向的本地 tokenizer 资产。不连接或写入
MySQL、Ollama、Qdrant，也不访问外部网络。

### RED 证据

命令：

    go test ./internal/contextretrieval -run 'Test(Merge|RenderContext|SelectWithin)'

结果：FAIL。失败原因是 `Merge`、`RenderContext`、`SelectWithinTokenBudget` 等目标
API 尚不存在。

### GREEN 证据

命令：

    go test ./internal/contextretrieval

结果：PASS。最终 race 与全量门禁在本节提交前执行。

### 实践行为

命令：

    go run ./cmd/context-merge-demo --config config/recent-chat.env --context-token-budget 160

本机被忽略的 `config/recent-chat.env` 最初缺少 `RECENT_CHAT_TOKENIZER_PATH`，demo
明确报错退出。确认本地资产存在后只补本机配置，不提交该文件。

实际输出：

    Memory candidates: 2
    Document candidates: 4
    Duplicate removed: true
    Merged source order: memory,document,document,document
    Selected source order: memory,document
    Dropped IDs: document:oversized,document:recent-window
    Used context tokens: 129
    Within budget: true
    Rendered retrieved_context:
    <retrieved_context>...</retrieved_context>

实践证明超长 `document:oversized` 被跳过后，后续较小 `document:token-budget` 仍能
进入预算；最终块由真实 qwen tokenizer 计为 129/160 tokens。

### Review 结论

本地 review 确认：两路只在各自来源内按 score 排序；ID 提供稳定同分顺序；去重只
针对已保留内容；输入 slice 与 Metadata 不被修改；标签属性和正文均转义；预算计算
完整渲染块并复验最终计数。

独立 review 代理超时且没有返回结果，已关闭。未因此跳过测试、race、vet、build、
格式和 diff 门禁。

## 第 28 节：Dual Retrieval 接入真实 Recent Chat

### 影响分析

本节修改 recent-chat 请求/响应与服务编排，注入真实 Ollama/Qdrant 客户端，新增服务
测试和最终 SOP，并更新学习/交接状态。

外部状态：

- 正常验证只读查询两个 Qdrant collection
- 专用 session `dual-retrieval-chat-stored-003` 成功写入一条 user 与一条 assistant 消息
- 不创建、修改或删除 Qdrant point/collection
- 不修改 `ollama_chat_memory`
- 临时错误 collection 只用于 GET/query，未创建；本机配置已恢复

### RED 证据

命令：

    go test ./internal/recentchat -run 'Test(ChatRequestRetrieval|ServiceRetrieval)'

修复测试自身 composite literal 语法后，有效 RED 因 `ChatRequest` 新字段、
`NewServiceWithContextRetrieval` 和 `ChatContext` 不存在而 FAIL。

### GREEN 证据

命令：

    go test ./internal/recentchat ./internal/contextretrieval ./internal/memoryitem
    go test -race ./internal/recentchat ./internal/contextretrieval ./internal/memoryitem

结果：PASS。旧 recentchat 全包回归也通过。

### 真实正常链路

使用本地 MySQL、qwen:7b、bge-m3 和两个 Qdrant collection。实际：

- memory=1，document=2
- retrieved context=330/512 tokens
- warning=0，answer 非空
- 同 session 后续请求读取 `used_messages=2`，确认 user/assistant 已写 MySQL

原计划 512 output reserve 的首次请求没有在可观测时间内取得响应；本机 qwen 弱模型
验证改为 128 output reserve，retrieval context budget 保持 512，约 21 秒完成。

### Scope 与故障隔离

- `missing-course`：memory=1、document=0、warning=0、answer=Go
- 不存在 document collection：memory=1、document=0、document warning=1、answer=Go
- 测试后两个服务进程已停止，配置恢复正常 document collection

### Review 重点

最终 review 检查旧请求兼容、request context 透传、fixed input 先包含 retrieval、summary
只组合一次、service ownership 二次校验、warning/hard failure 分界、Ollama 前失败不写消息、
启动无 collection 写操作和响应不暴露向量。

Review 补强：把 service ownership 测试扩展为跨 user 与跨 knowledge scope 两种情况，
均确认 Ollama 调用数为 0；同时修正 handoff guide 遗留的不便移植的本机绝对路径。

独立 review 代理超时且无返回结果，已关闭；最终全量门禁与完成审计不因此省略。
