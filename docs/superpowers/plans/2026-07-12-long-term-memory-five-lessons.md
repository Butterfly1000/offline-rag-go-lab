# Long-term Memory Item Five Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成第 19-23 节，使长期记忆从候选定义、真实 Ollama 提取、确定性决策、MySQL 事实存储到 Qdrant 语义检索形成可运行闭环。

**Architecture:** `internal/memoryitem` 通过小接口组合 validator、extractor、resolver、MySQL store、embedder 和 Qdrant indexer，不依赖 `recentchat` 或 `sessionsummary`。MySQL 是带来源证据的唯一事实源，Qdrant 是按用户隔离、可从 MySQL 重建的 1024 维语义索引。

**Tech Stack:** Go 1.23、MySQL、Ollama `/api/chat` JSON schema、Ollama `/api/embed`、`qwen:7b`、`bge-m3`、Qdrant 1.18 REST API、Go testing

## Global Constraints

- 只修改 `/offline-rag-go-lab`。
- 每节执行 RED -> GREEN -> 实践/SOP -> review -> 独立 commit。
- 不执行 `git push`。
- 不修改或复用现有 `ollama_chat_memory` collection。
- 不把 memory 自动注入 `/chat`，不做 memory/document retrieval 融合。
- MySQL schema 和 Qdrant collection/point 写入前必须说明影响并获得用户许可。
- MySQL 凭据和本机路径只放 Git 忽略的本地配置，不使用环境变量。
- 所有 MySQL 与 Qdrant 查询必须带 `user_id` 边界。
- Qdrant 失败不能反向覆盖 MySQL；MySQL 是事实源。
- 文档路径使用 `/offline-rag-go-lab/...`，不写 `/Users/...`。
- 非阻塞优化记录到 `docs/teaching/00-optimization-backlog.md`，不扩大当前五节范围。

---

### Task 19: Memory Item 边界、类型与来源校验

**Files:**
- Create: `internal/memoryitem/types.go`
- Create: `internal/memoryitem/validate.go`
- Create: `internal/memoryitem/validate_test.go`
- Create: `cmd/memory-item-validate-demo/main.go`
- Create: `docs/teaching/memory-item-validation-sop.md`
- Modify: `docs/teaching/00-long-term-memory-batch-operation-log.md`

**Interfaces:**
- Consumes: 无；本节是后续四节的领域基线。
- Produces: `SourceMessage`、`Candidate`、`Item`、`ValidateAndNormalizeCandidate`。

- [x] **Step 1: 写候选校验 RED 测试**

测试表至少包含：合法 upsert、合法 forget、未知 operation/kind、非法 key、空 upsert value、confidence 越界、空来源、未知来源、跨用户来源、assistant-only 来源和重复来源 ID。

```go
func TestValidateAndNormalizeCandidate(t *testing.T) {
    messages := []SourceMessage{{
        ID: 101, SessionID: "memory-validation", UserID: "u-001",
        Role: "user", Content: "这个项目使用 Go。",
    }}
    got, err := ValidateAndNormalizeCandidate("u-001", Candidate{
        Operation: OperationUpsert,
        Kind: KindProjectFact,
        Key: " Implementation Language ",
        Value: " Go ", Confidence: 0.95,
        SourceMessageIDs: []int64{101},
    }, messages)
    if err != nil { t.Fatal(err) }
    if got.Key != "implementation_language" || got.Value != "Go" {
        t.Fatalf("normalized candidate = %#v", got)
    }
}
```

- [x] **Step 2: 运行 RED**

Run: `go test ./internal/memoryitem -run TestValidateAndNormalizeCandidate`

Expected: FAIL，因为 `SourceMessage`、`Candidate` 和 `ValidateAndNormalizeCandidate` 尚不存在。

- [x] **Step 3: 实现最小领域类型和校验器**

```go
type Operation string
const (
    OperationUpsert Operation = "upsert"
    OperationForget Operation = "forget"
)

type Kind string
const (
    KindIdentity    Kind = "identity"
    KindPreference  Kind = "preference"
    KindProjectFact Kind = "project_fact"
    KindGoal        Kind = "goal"
    KindConstraint  Kind = "constraint"
)

type Status string
const (
    StatusActive    Status = "active"
    StatusForgotten Status = "forgotten"
)

func ValidateAndNormalizeCandidate(userID, sessionID string, candidate Candidate, messages []SourceMessage) (Candidate, error)
```

实现必须：规范化 key 为 snake_case、裁剪 value、验证枚举和 confidence、去重并保持来源 ID 顺序、确认每个来源属于当前用户、当前 session 且 role 为 user。forget 不要求 value，但必须有明确来源。

- [x] **Step 4: 运行 GREEN 和完整 package 测试**

Run: `go test ./internal/memoryitem`

Expected: PASS，所有边界测试通过。

- [x] **Step 5: 添加纯 Go 实践命令**

Run: `go run ./cmd/memory-item-validate-demo`

Expected output 包含：

```text
Valid candidate: project_fact/implementation_language=Go
Sources: 101
Rejected assistant-only source: source message 102 must have role user
```

- [x] **Step 6: 编写 SOP、记录影响并 review**

SOP 必须讲清 session summary 与 memory item 边界、五种 kind、来源约束和为什么模型输出仍需 Go 校验。操作日志记录本节只改纯 Go 代码和文档，未访问 MySQL/Ollama/Qdrant。

执行：

```bash
go test ./...
go test -race ./internal/memoryitem
go vet ./...
go build ./cmd/...
git diff --check
```

- [x] **Step 7: 独立提交**

```bash
git add internal/memoryitem cmd/memory-item-validate-demo docs/teaching/memory-item-validation-sop.md docs/teaching/00-long-term-memory-batch-operation-log.md
git commit -m "feat: validate long-term memory candidates"
```

### Task 20: 真实 Ollama 结构化候选提取

**Files:**
- Create: `internal/memoryitem/prompt.go`
- Create: `internal/memoryitem/prompt_test.go`
- Create: `internal/memoryitem/extractor.go`
- Create: `internal/memoryitem/extractor_test.go`
- Modify: `internal/recentchat/ollama.go`
- Create: `internal/recentchat/ollama_json_test.go`
- Create: `cmd/memory-extract-demo/main.go`
- Create: `docs/teaching/memory-item-extraction-sop.md`
- Modify: `docs/teaching/00-long-term-memory-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 19 `SourceMessage`、`Candidate`、`ValidateAndNormalizeCandidate`。
- Produces: `StructuredGenerator`、`Extractor`、`ExtractRequest`、`ExtractResult`，以及 `HTTPOllamaClient.GenerateJSON`。

- [x] **Step 1: 写 prompt、解析和 Ollama adapter RED 测试**

覆盖：消息顺序和 ID、summary 辅助区、XML-like 正文边界、JSON schema 字段、非法 JSON、未知来源、模型错误、空模型、非法 max token 和合法候选。

```go
type fakeStructuredGenerator struct { response []byte }
func (f fakeStructuredGenerator) GenerateJSON(string, string, string, json.RawMessage, int) ([]byte, error) {
    return f.response, nil
}

func TestExtractorRejectsUnknownSource(t *testing.T) {
    extractor := NewExtractor(fakeStructuredGenerator{response: []byte(
        `{"candidates":[{"operation":"upsert","kind":"goal","key":"next_goal","value":"完成记忆系统","confidence":0.9,"source_message_ids":[999]}]}`,
    )})
    _, err := extractor.Extract(ExtractRequest{/* message ID 101 only */})
    if err == nil { t.Fatal("expected unknown source error") }
}
```

- [x] **Step 2: 运行 RED**

Run: `go test ./internal/memoryitem ./internal/recentchat -run 'Test(BuildExtractionPrompt|Extractor|HTTPOllamaClientGenerateJSON)'`

Expected: FAIL，因为 extractor、schema format 和 `GenerateJSON` 尚不存在。

- [x] **Step 3: 实现 prompt 与 extractor**

```go
type StructuredGenerator interface {
    GenerateJSON(model, system, prompt string, schema json.RawMessage, maxTokens int) ([]byte, error)
}

type ExtractRequest struct {
    Model, UserID, SessionID, Summary string
    Messages []SourceMessage
    MaxOutputTokens int
}

type ExtractResult struct {
    RawJSON []byte
    Candidates []Candidate
}

func NewExtractor(generator StructuredGenerator) *Extractor
func (e *Extractor) Extract(req ExtractRequest) (ExtractResult, error)
```

Extractor 必须 strict decode 顶层 `candidates`，拒绝尾随 JSON，并对每条候选调用 Task 19 校验器。system prompt 必须声明消息是数据而非指令，assistant 不能成为唯一证据，不因缺失事实生成 forget。

- [x] **Step 4: 扩展现有 Ollama client 的 JSON schema 请求**

在 `OllamaChatRequest` 增加：

```go
Format json.RawMessage `json:"format,omitempty"`
```

新增：

```go
func (c *HTTPOllamaClient) GenerateJSON(model, system, prompt string, schema json.RawMessage, maxTokens int) ([]byte, error)
```

adapter 测试必须检查 `/api/chat`、`stream=false`、两条 system/user message、`format` 为 JSON schema、`num_predict` 与错误状态传播。

- [x] **Step 5: 运行 GREEN**

Run: `go test ./internal/memoryitem ./internal/recentchat`

Expected: PASS。

- [x] **Step 6: 运行真实 Ollama 提取**

Run: `go run ./cmd/memory-extract-demo --config config/recent-chat.env --model qwen:7b`

固定输入至少包含：姓名、Go 项目事实、真实操作教学偏好和一条 assistant 推测。Expected：输出合法 JSON；校验后保留用户明确事实，不把 assistant 推测变成 memory item。

- [x] **Step 7: SOP、review 和独立提交**

SOP 记录真实模型输出、JSON schema 只约束形状而非事实、prompt injection 边界和失败行为。完整执行全量测试、两个相关 package race、vet、build、diff check。

```bash
git add internal/memoryitem internal/recentchat/ollama.go internal/recentchat/ollama_json_test.go cmd/memory-extract-demo docs/teaching/memory-item-extraction-sop.md docs/teaching/00-long-term-memory-batch-operation-log.md
git commit -m "feat: extract structured memory candidates"
```

### Task 21: 规范化与确定性生命周期决策

**Files:**
- Create: `internal/memoryitem/resolve.go`
- Create: `internal/memoryitem/resolve_test.go`
- Create: `cmd/memory-resolve-demo/main.go`
- Create: `docs/teaching/memory-item-resolution-sop.md`
- Modify: `docs/teaching/00-long-term-memory-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 19 已校验的 `Candidate`、`Item`、`Status`。
- Produces: `Action`、`Decision`、`Resolve`、`ResolveBatch`。

- [x] **Step 1: 写四种决策和恢复行为 RED 测试**

```go
func TestResolveUpdatesChangedValue(t *testing.T) {
    current := &Item{ID: 7, UserID: "u-001", Kind: KindProjectFact,
        Key: "implementation_language", Value: "PHP", Status: StatusActive, Version: 2}
    decision, err := Resolve(current, Candidate{Operation: OperationUpsert,
        Kind: KindProjectFact, Key: "implementation_language", Value: "Go"})
    if err != nil { t.Fatal(err) }
    if decision.Action != ActionUpdate || decision.Next.Version != 3 || decision.Next.Value != "Go" {
        t.Fatalf("decision = %#v", decision)
    }
}
```

覆盖 INSERT、UPDATE、NOOP、FORGET、forgotten 恢复、重复 forget、key/kind 不匹配、非法 current version，以及 batch 的稳定来源顺序和冲突可观测结果。

- [x] **Step 2: 运行 RED**

Run: `go test ./internal/memoryitem -run 'TestResolve'`

Expected: FAIL，因为 resolver 类型和函数尚不存在。

- [x] **Step 3: 实现确定性 resolver**

```go
type Action string
const (
    ActionInsert Action = "insert"
    ActionUpdate Action = "update"
    ActionNoop   Action = "noop"
    ActionForget Action = "forget"
)

type Decision struct {
    Action Action
    Current *Item
    Next Item
    Candidate Candidate
    Reason string
}

func Resolve(current *Item, candidate Candidate) (Decision, error)
func ResolveBatch(current map[string]Item, candidates []Candidate) ([]Decision, error)
```

NOOP 不增加 version；insert 从 version 1 开始；update、forget 和恢复增加 version；batch identity key 使用 `kind + "\x00" + key`，并按最小来源 ID 和原始顺序稳定处理。

- [x] **Step 4: GREEN 与实践命令**

Run: `go test ./internal/memoryitem`

Run: `go run ./cmd/memory-resolve-demo`

Expected output 依次展示：INSERT Go、NOOP Go、UPDATE Rust、FORGET、恢复 UPDATE Go，并打印每一步 version。

- [x] **Step 5: SOP、review 和独立提交**

SOP 必须说明“LLM 提候选，Go 决策”的职责分离、相同 key 去重边界、为什么缺失候选不能删除。完整执行全量测试、race、vet、build、diff check。

```bash
git add internal/memoryitem cmd/memory-resolve-demo docs/teaching/memory-item-resolution-sop.md docs/teaching/00-long-term-memory-batch-operation-log.md
git commit -m "feat: resolve memory item lifecycle"
```

### Task 22: MySQL Memory Store 与来源证据事务

**Files:**
- Create: `internal/memoryitem/store.go`
- Create: `internal/memoryitem/store_test.go`
- Create: `internal/memoryitem/store_mysql.go`
- Create: `internal/memoryitem/store_mysql_test.go`
- Create: `sql/memory_items.sql`
- Create: `cmd/memory-store-demo/main.go`
- Create: `cmd/memory-store-demo/main_test.go`
- Create: `docs/teaching/memory-item-store-sop.md`
- Modify: `config/recent-chat.env.example`
- Modify: `docs/teaching/00-long-term-memory-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 21 `Resolve` 和 `Decision`，Task 19 candidate/source types。
- Produces: `MemoryStore`、`ApplyRequest`、`ApplyResult`、`Evidence`、`NewMySQLMemoryStore`。

- [ ] **Step 1: 写事务行为 RED 测试**

```go
type MemoryStore interface {
    Get(ctx context.Context, userID string, kind Kind, key string) (Item, bool, error)
    Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
    ListActive(ctx context.Context, userID string) ([]Item, error)
}

type ApplyRequest struct {
    UserID, SessionID string
    Candidate Candidate
    SourceMessages []SourceMessage
}
```

测试覆盖：首次 insert+evidence、相同值 noop+新 evidence、重复 evidence 幂等、update/version、forget、恢复、跨用户查询、事务回滚、零影响 version conflict、duplicate key conflict 和 SQL 中 user filter/`FOR UPDATE`。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/memoryitem -run 'Test(Store|MySQLMemory)'`

Expected: FAIL，因为 store 和 MySQL adapter 尚不存在。

- [ ] **Step 3: 实现可测试事务边界和业务 store**

```go
var ErrVersionConflict = errors.New("memory item version conflict")

type Evidence struct {
    ItemID int64
    UserID, SessionID, Role string
    MessageID int64
    Operation Operation
    Text string
}

type ApplyResult struct {
    Decision Decision
    Item Item
    EvidenceInserted int
}
```

`Apply` 必须在同一事务中读取 `SELECT ... FOR UPDATE`、调用 resolver、写 item 状态并插入 evidence。item 状态未变化时不加 version；evidence 使用唯一键实现幂等。事务失败不返回伪成功结果。

- [ ] **Step 4: 增加 MySQL schema**

`memory_items` 使用唯一键 `(user_id, kind, memory_key)`，`memory_item_evidence` 使用唯一键 `(memory_item_id, source_session_id, source_message_id, operation)`，并通过 foreign key 关联 item。两个表使用 InnoDB/utf8mb4；不删除 item，只改变 status。

- [ ] **Step 5: GREEN 和静态 review**

Run: `go test ./internal/memoryitem ./cmd/memory-store-demo`

Expected: PASS。检查 SQL 所有查找/更新均包含 user_id，事务 commit/rollback 都有测试。

- [ ] **Step 6: 在真实外部写入前停止并请求许可**

向用户说明：将创建两张表，并只写专用 `memory-store-demo-user` 测试数据；不会修改现有消息、summary 或 Qdrant。未获许可时只保留单元测试和 schema，不宣称真实闭环完成。

- [ ] **Step 7: 获批后执行真实 MySQL 实践**

Run: `go run ./cmd/memory-store-demo --config config/recent-chat.env --apply-schema`

命令对主要事实按顺序执行 upsert Go、重复 upsert、改为 Rust、改回 Go，Expected：action 为 INSERT/NOOP/UPDATE/UPDATE，version 为 1/1/2/3。再插入并 forget 一条 `temporary_tool`，Expected：INSERT/FORGET、version 1/2。这样既验证遗忘，又保留 active 项供下一节检索；所有 evidence 可追溯，重复运行不重复插入相同来源。

- [ ] **Step 8: SOP、review 和独立提交**

SOP 记录表职责、事务边界、真实行结果和本地配置方式。完整执行全量测试、相关 package/command race、vet、build、diff check，并检查 Git 中无 DSN。

```bash
git add internal/memoryitem sql/memory_items.sql cmd/memory-store-demo config/recent-chat.env.example docs/teaching/memory-item-store-sop.md docs/teaching/00-long-term-memory-batch-operation-log.md
git commit -m "feat: persist evidenced memory items"
```

### Task 23: bge-m3 Embedding 与 Qdrant 用户隔离检索

**Files:**
- Create: `internal/memoryitem/embed.go`
- Create: `internal/memoryitem/embed_test.go`
- Create: `internal/memoryitem/qdrant.go`
- Create: `internal/memoryitem/qdrant_test.go`
- Create: `cmd/memory-qdrant-demo/main.go`
- Create: `cmd/memory-qdrant-demo/main_test.go`
- Create: `docs/teaching/memory-item-qdrant-sop.md`
- Modify: `config/recent-chat.env.example`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`
- Modify: `docs/teaching/00-long-term-memory-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 22 `MemoryStore.ListActive`、`Item` 和 item status/version。
- Produces: `Embedder`、`HTTPOllamaEmbedder`、`QdrantIndexer`、`SearchResult`。

- [ ] **Step 1: 写 embedding client RED 测试**

```go
type Embedder interface {
    Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}
```

httptest 覆盖 `/api/embed` 请求、多个输入的一一对应、非 2xx、空 vector、NaN/Inf、不同维度、空模型和空文本。

- [ ] **Step 2: 写 Qdrant client RED 测试**

```go
type SearchResult struct {
    ItemID int64
    Score float64
    UserID string
    Kind Kind
    Key, Value string
    Version int64
}

func (q *QdrantIndexer) EnsureCollection(ctx context.Context, vectorSize int) error
func (q *QdrantIndexer) Upsert(ctx context.Context, item Item, vector []float32, model string) error
func (q *QdrantIndexer) Delete(ctx context.Context, itemID int64) error
func (q *QdrantIndexer) Search(ctx context.Context, userID string, kind Kind, vector []float32, limit int) ([]SearchResult, error)
```

测试必须检查：集合名 URL escape、1024/Cosine 配置、已有配置不匹配报错、point ID=item ID、payload version/model、search 强制 user_id filter、可选 kind filter、delete wait=true、错误 body 截断和超时传播。

- [ ] **Step 3: 运行 RED**

Run: `go test ./internal/memoryitem -run 'Test(HTTPOllamaEmbedder|QdrantIndexer)'`

Expected: FAIL，因为 embedding/Qdrant 类型尚不存在。

- [ ] **Step 4: 实现标准库 HTTP clients**

Ollama 使用 `POST /api/embed`。Qdrant 使用：

```text
GET  /collections/{collection}
PUT  /collections/{collection}
PUT  /collections/{collection}/index?wait=true
PUT  /collections/{collection}/points?wait=true
POST /collections/{collection}/points/query
POST /collections/{collection}/points/delete?wait=true
```

创建 `user_id` 和 `kind` keyword payload index。集合存在时只核对 `size=1024`、`distance=Cosine`；不匹配时返回错误，不自动删除。

- [ ] **Step 5: GREEN 与全量单元测试**

Run: `go test ./internal/memoryitem ./cmd/memory-qdrant-demo`

Expected: PASS，httptest 能证明 request body 和 user filter。

- [ ] **Step 6: 在真实 Qdrant 写入前停止并请求许可**

向用户说明：将创建 `offline_rag_memory_items_v1`、两个 payload index，并写入专用测试用户 points；现有 `ollama_chat_memory` 不读取 payload、不修改。未获许可时不能宣称 Qdrant 闭环完成。

- [ ] **Step 7: 获批后运行真实 embedding 与 Qdrant 实践**

Run: `go run ./cmd/memory-qdrant-demo --config config/recent-chat.env --ensure-collection`

Expected：

```text
Embedding model: bge-m3
Vector dimension: 1024
Collection: offline_rag_memory_items_v1 (Cosine)
Upserted active items: <positive count>
Search filter user_id=memory-store-demo-user
Top result: project_fact/implementation_language
Forgotten item present: false
```

再使用第二个测试用户写入相近文本，确认第一个用户检索结果不包含第二个用户 point。

- [ ] **Step 8: 更新教学进度、backlog 和 SOP**

SOP 讲清 embedding 与生成模型区别、维度来自真实响应、Cosine、payload filter、MySQL/Qdrant 主从边界和 curl 等价请求。学习进度把第 3 层标记为已完成独立提取/存储/检索闭环，下一章指向 memory retrieval 与 document retrieval 合并。backlog 记录 outbox/rebuild worker、跨 key 语义去重和索引漂移扫描。

- [ ] **Step 9: 最终 review、完整 gate 和独立提交**

执行：

```bash
go test ./...
go test -race ./internal/memoryitem ./internal/recentchat ./cmd/memory-store-demo ./cmd/memory-qdrant-demo
go vet ./...
go build ./cmd/...
git diff --check
git status --short
```

检查：无凭据、无本机绝对路径、未修改旧 collection、真实 evidence/point 仅属于专用测试用户、五个实现 commit 齐全。

```bash
git add internal/memoryitem cmd/memory-qdrant-demo config/recent-chat.env.example docs/teaching/memory-item-qdrant-sop.md docs/teaching/00-learning-status.md docs/teaching/00-optimization-backlog.md docs/teaching/00-long-term-memory-batch-operation-log.md
git commit -m "feat: index and search long-term memories"
```

## Completion Audit

- [ ] 设计/计划 commit 之后恰好有第 19-23 节五个实现 commit。
- [ ] 五份 SOP 都包含代码行为、真实命令、结果解释、生产边界和下一步。
- [ ] 操作影响日志记录每次外部访问、写入范围、review 发现和修复。
- [ ] `go test ./...`、相关 race、`go vet ./...`、`go build ./cmd/...`、`git diff --check` 全部通过。
- [ ] 真实 `qwen:7b` 结构化提取通过。
- [ ] 真实 MySQL insert/update/noop/forget/evidence 通过。
- [ ] 真实 `bge-m3` 1024 维 embedding 和 Qdrant 用户隔离检索通过。
- [ ] 现有 `ollama_chat_memory` 和非测试数据未修改。
- [ ] 工作区干净；只 commit，不 push。
