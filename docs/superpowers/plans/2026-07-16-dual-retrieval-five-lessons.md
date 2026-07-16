# Dual Retrieval Five Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete lessons 24-28 so real user-scoped memory retrieval and real knowledge-scope document retrieval can be combined, token-budgeted and injected into recent-chat.

**Architecture:** A new internal/contextretrieval package owns common hit validation, document Qdrant access, dual-source orchestration, deterministic merging and exact tokenizer budgeting. Existing memoryitem Qdrant remains the memory index, while recentchat receives the new behavior through an optional interface so old requests remain unchanged.

**Tech Stack:** Go 1.23 module semantics on Go 1.26 runtime, Ollama /api/embed and /api/chat, bge-m3, qwen:7b, Qdrant 1.18 REST API, MySQL, github.com/sugarme/tokenizer, Go testing

## Global Constraints

- Modify only /offline-rag-go-lab.
- Execute RED -> GREEN -> practice/SOP -> review -> independent commit for each lesson.
- Do not run git push.
- Do not record implementation completion as user learning completion.
- Use config/recent-chat.env rather than required shell environment variables.
- Never modify or reuse ollama_chat_memory.
- Keep offline_rag_memory_items_v1 as the existing memory collection.
- Create document points only in offline_rag_document_chunks_v1.
- Memory queries must require user_id; document queries must require knowledge_scope.
- Returned payload ownership must be revalidated after Qdrant search.
- Infrastructure failures may degrade to warnings in chat; ownership and malformed-payload errors remain hard failures.
- MySQL remains the memory fact source; Qdrant failures never update or overwrite MySQL.
- Documentation paths use /offline-rag-go-lab rather than /Users paths.
- Non-blocking production enhancements go to docs/teaching/00-optimization-backlog.md.

---

### Task 24: Unified Retrieval Hit and Ownership Boundary

**Files:**
- Create: `internal/contextretrieval/types.go`
- Create: `internal/contextretrieval/validate.go`
- Create: `internal/contextretrieval/validate_test.go`
- Create: `internal/contextretrieval/errors.go`
- Create: `internal/contextretrieval/errors_test.go`
- Create: `cmd/context-hit-demo/main.go`
- Create: `docs/teaching/context-hit-boundary-sop.md`
- Create: `docs/teaching/00-dual-retrieval-batch-operation-log.md`

**Interfaces:**
- Consumes: no runtime dependency; this task defines the shared retrieval domain.
- Produces: `Source`, `Hit`, `ValidateHit`, `SourceError`, `InfrastructureFailure`, `IntegrityFailure`, `IsInfrastructureFailure`.

- [x] **Step 1: Write failing ownership and error-classification tests**

Create table tests with these exact cases:

```go
func TestValidateHitEnforcesSourceOwnership(t *testing.T) {
    validMemory := Hit{
        Source: SourceMemory, ID: "memory:7", Content: "project_fact/language: Go",
        Score: 0.91, UserID: "u-001",
    }
    got, err := ValidateHit(validMemory)
    if err != nil {
        t.Fatal(err)
    }
    if got.UserID != "u-001" || got.KnowledgeScope != "" {
        t.Fatalf("validated memory hit = %#v", got)
    }

    mixed := validMemory
    mixed.KnowledgeScope = "project-a"
    if _, err := ValidateHit(mixed); err == nil {
        t.Fatal("expected mixed ownership error")
    }
}
```

Cover valid memory, valid document, empty ID/content, non-finite score, missing user,
missing scope, memory with scope, document with user and unknown source.

Add error tests proving:

```go
infra := InfrastructureFailure(SourceMemory, errors.New("timeout"))
if !IsInfrastructureFailure(infra) { t.Fatal("want infrastructure failure") }

integrity := IntegrityFailure(SourceDocument, errors.New("wrong scope"))
if IsInfrastructureFailure(integrity) { t.Fatal("integrity must remain hard") }
```

- [x] **Step 2: Run RED**

Run:

```bash
go test ./internal/contextretrieval -run 'Test(ValidateHit|SourceError)'
```

Expected: FAIL because the package and types do not exist.

- [x] **Step 3: Implement the domain and typed failures**

Use these exact public shapes:

```go
type Source string

const (
    SourceMemory   Source = "memory"
    SourceDocument Source = "document"
)

type Hit struct {
    Source         Source            `json:"source"`
    ID             string            `json:"id"`
    Content        string            `json:"content"`
    Score          float64           `json:"score"`
    UserID         string            `json:"user_id,omitempty"`
    KnowledgeScope string            `json:"knowledge_scope,omitempty"`
    Kind           string            `json:"kind,omitempty"`
    Title          string            `json:"title,omitempty"`
    SourceRef      string            `json:"source_ref,omitempty"`
    Metadata       map[string]string `json:"metadata,omitempty"`
}

func ValidateHit(hit Hit) (Hit, error)
```

Validation trims strings, copies Metadata, requires finite score and enforces:

```go
switch hit.Source {
case SourceMemory:
    require(hit.UserID != "")
    require(hit.KnowledgeScope == "")
case SourceDocument:
    require(hit.KnowledgeScope != "")
    require(hit.UserID == "")
default:
    return Hit{}, fmt.Errorf("unknown retrieval source %q", hit.Source)
}
```

Implement typed failures:

```go
type FailureKind string

const (
    FailureInfrastructure FailureKind = "infrastructure"
    FailureIntegrity      FailureKind = "integrity"
)

type SourceError struct {
    Source Source
    Kind   FailureKind
    Err    error
}

func (e *SourceError) Error() string
func (e *SourceError) Unwrap() error
func InfrastructureFailure(source Source, err error) error
func IntegrityFailure(source Source, err error) error
func IsInfrastructureFailure(err error) bool
```

- [x] **Step 4: Run GREEN and package race test**

Run:

```bash
go test ./internal/contextretrieval
go test -race ./internal/contextretrieval
```

Expected: PASS.

- [x] **Step 5: Add and run the pure Go practice command**

The demo constructs one memory hit, one document hit and one mixed hit.

Run:

```bash
go run ./cmd/context-hit-demo
```

Expected output contains:

```text
Valid memory: source=memory user_id=u-001
Valid document: source=document knowledge_scope=offline-rag-course
Rejected mixed ownership: memory hit must not carry knowledge_scope
```

- [x] **Step 6: Write the lesson SOP and operation log**

The SOP explains:

- why a unified shape must retain Source
- why user_id and knowledge_scope are not interchangeable
- why validation runs after Qdrant returns data
- why infrastructure and integrity failures have different chat behavior
- current implementation versus production tenant authorization

The operation log records no MySQL, Ollama or Qdrant access.

- [x] **Step 7: Review and verify**

Run:

```bash
gofmt -w internal/contextretrieval/*.go cmd/context-hit-demo/*.go
go test ./...
go test -race ./internal/contextretrieval
go vet ./...
go build ./cmd/...
git diff --check
```

Review the diff for source ownership, copied maps, finite scores, comments and full machine paths.

- [x] **Step 8: Commit lesson 24**

```bash
git add internal/contextretrieval cmd/context-hit-demo docs/teaching/context-hit-boundary-sop.md docs/teaching/00-dual-retrieval-batch-operation-log.md
git commit -m "feat: define dual retrieval hit boundaries"
```

---

### Task 25: Real Document Qdrant Index and Scope-Filtered Search

**Files:**
- Create: `internal/contextretrieval/document.go`
- Create: `internal/contextretrieval/document_test.go`
- Create: `internal/contextretrieval/document_qdrant.go`
- Create: `internal/contextretrieval/document_qdrant_test.go`
- Create: `cmd/document-qdrant-demo/main.go`
- Create: `docs/teaching/document-qdrant-sop.md`
- Modify: `config/recent-chat.env.example`
- Modify: `docs/teaching/00-dual-retrieval-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 24 `Hit`, `SourceDocument`, `ValidateHit`, typed source errors; memoryitem `Embedder`.
- Produces: `DocumentChunk`, `DocumentQdrant`, `DeterministicDocumentPointID`, `EnsureCollection`, `Upsert`, `Search`.

- [x] **Step 1: Write document identity and validation RED tests**

Use this public type:

```go
type DocumentChunk struct {
    KnowledgeScope string
    DocumentID     string
    ChunkID        string
    Title          string
    SourceRef      string
    Text           string
}
```

Test that valid chunks are trimmed, empty ownership/identity/text is rejected, and
the point ID is stable:

```go
first, err := DeterministicDocumentPointID("offline-rag-course", "chunk-001")
if err != nil { t.Fatal(err) }
second, _ := DeterministicDocumentPointID("offline-rag-course", "chunk-001")
if first != second { t.Fatalf("point IDs differ: %q %q", first, second) }
if first == mustPointID("another-scope", "chunk-001") {
    t.Fatal("scope must participate in point identity")
}
```

The implementation uses SHA256 of `scope + "\\x00" + chunkID`, sets UUID version
and variant bits, and formats a lowercase UUID string.

- [x] **Step 2: Write Qdrant RED tests with httptest.Server**

Cover exact request behavior:

- GET collection followed by PUT create with size 1024 and Cosine
- keyword indexes for knowledge_scope and document_id
- deterministic point ID and complete payload on upsert
- query request with mandatory knowledge_scope filter
- result conversion to SourceDocument Hit
- cross-scope payload returned by Qdrant becomes IntegrityFailure
- HTTP/non-success/context errors become InfrastructureFailure
- mismatched payload chunk ID, empty text and non-finite score are rejected

- [x] **Step 3: Run RED**

```bash
go test ./internal/contextretrieval -run 'Test(Document|Deterministic)'
```

Expected: FAIL because document types and client do not exist.

- [x] **Step 4: Implement document identity and Qdrant client**

Use:

```go
type DocumentQdrant struct {
    baseURL    string
    collection string
    client     *http.Client
}

func NewDocumentQdrant(baseURL, collection string) *DocumentQdrant
func (q *DocumentQdrant) EnsureCollection(ctx context.Context, vectorSize int) error
func (q *DocumentQdrant) Upsert(
    ctx context.Context,
    chunk DocumentChunk,
    vector []float32,
    embeddingModel string,
) error
func (q *DocumentQdrant) Search(
    ctx context.Context,
    knowledgeScope string,
    vector []float32,
    limit int,
) ([]Hit, error)
```

The HTTP client timeout is 30 seconds. Upsert payload includes content_hash as the
hex SHA256 of the trimmed text. Search uses `/collections/{name}/points/query`
with `with_payload=true`, `with_vector=false` and this mandatory filter:

```json
{"must":[{"key":"knowledge_scope","match":{"value":"offline-rag-course"}}]}
```

Qdrant response payload is revalidated before constructing each Hit.

- [x] **Step 5: Run GREEN**

```bash
go test ./internal/contextretrieval
go test -race ./internal/contextretrieval
```

Expected: PASS.

- [x] **Step 6: Add the real document Qdrant demo**

The command reads these file-config keys:

```text
OLLAMA_BASE_URL
OLLAMA_EMBED_MODEL
QDRANT_BASE_URL
QDRANT_DOCUMENT_COLLECTION
```

It refuses any collection name other than
`offline_rag_document_chunks_v1` and requires `--apply`.

Use deterministic fixtures in scopes `offline-rag-course` and
`another-course`, embed them with bge-m3, ensure the collection, upsert and
query both scopes. The command verifies the first scope never returns the second.

Add to the checked-in example:

```text
QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
```

- [x] **Step 7: Run the real practice**

First verify services without writing:

```bash
curl -sS --max-time 10 http://127.0.0.1:11434/api/tags
curl -sS --max-time 10 http://127.0.0.1:6333/collections
```

Then run the authorized idempotent write:

```bash
go run ./cmd/document-qdrant-demo --config config/recent-chat.env --apply
```

Expected output contains:

```text
Embedding model: bge-m3
Vector dimension: 1024
Collection: offline_rag_document_chunks_v1 (Cosine)
Cross-scope point present: false
Idempotent point IDs: true
```

- [x] **Step 8: Write SOP, operation impact and production boundary**

Document the curl/command, collection schema, deterministic IDs, mandatory scope
filter, payload indexes, result checks and actual output. Record that this lesson
writes only fixed idempotent points to the new document collection.

Record ingestion worker, aliases and rebuild jobs as production boundaries in
the SOP; the shared optimization backlog is updated in lesson 27.

- [x] **Step 9: Review and verify**

```bash
gofmt -w internal/contextretrieval/*.go cmd/document-qdrant-demo/*.go
go test ./...
go test -race ./internal/contextretrieval
go vet ./...
go build ./cmd/...
git diff --check
```

Review every Qdrant path, filter, response ownership check, timeout, collection
guard, config example and secret scan.

- [x] **Step 10: Commit lesson 25**

```bash
git add internal/contextretrieval cmd/document-qdrant-demo config/recent-chat.env.example docs/teaching/document-qdrant-sop.md docs/teaching/00-dual-retrieval-batch-operation-log.md
git commit -m "feat: index and search document chunks"
```

---

### Task 26: Shared Embedding, Parallel Dual Retrieval and Failure Isolation

**Files:**
- Create: `internal/contextretrieval/memory_adapter.go`
- Create: `internal/contextretrieval/memory_adapter_test.go`
- Create: `internal/contextretrieval/dual.go`
- Create: `internal/contextretrieval/dual_test.go`
- Create: `cmd/dual-retrieval-demo/main.go`
- Create: `docs/teaching/dual-retrieval-sop.md`
- Create: `internal/memoryitem/qdrant_error.go`
- Create: `internal/memoryitem/qdrant_error_test.go`
- Modify: `internal/memoryitem/qdrant.go`
- Modify: `docs/teaching/00-dual-retrieval-batch-operation-log.md`

**Interfaces:**
- Consumes: Task 25 DocumentQdrant; existing memoryitem Embedder, QdrantIndexer and SearchResult.
- Produces: `MemoryQdrantSearcher`, `DualRequest`, `DualResult`, `DualRetriever`, memory Qdrant data-integrity classification.

- [x] **Step 1: Add RED tests for memory result adaptation**

Define a narrow internal dependency:

```go
type MemoryVectorSearcher interface {
    Search(ctx context.Context, userID string, kind memoryitem.Kind, vector []float32, limit int) ([]memoryitem.SearchResult, error)
}
```

Test conversion to a validated memory Hit with ID `memory:{itemID}`, Kind, key in
Metadata and the required UserID. Test that a returned different user or malformed
memory result becomes IntegrityFailure.

- [x] **Step 2: Add RED tests for memory Qdrant error classes**

Add an exported wrapper only for malformed response data:

```go
type QdrantDataError struct { Err error }
func (e *QdrantDataError) Error() string
func (e *QdrantDataError) Unwrap() error
func IsQdrantDataError(err error) bool
```

Wrap errors created while decoding or validating returned Qdrant point IDs,
payload ownership, kind, key, value, version and score. HTTP, timeout and context
errors remain ordinary infrastructure errors.

Tests prove `errors.As` survives outer `fmt.Errorf("%w")` wrapping.

- [x] **Step 3: Add dual retrieval RED tests**

Use:

```go
type QueryEmbedder interface {
    Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}

type MemorySearcher interface {
    Search(ctx context.Context, userID string, vector []float32, limit int) ([]Hit, error)
}

type DocumentSearcher interface {
    Search(ctx context.Context, knowledgeScope string, vector []float32, limit int) ([]Hit, error)
}

type DualRequest struct {
    Query           string
    UserID          string
    KnowledgeScope  string
    UseMemory       bool
    UseDocuments    bool
    MemoryLimit     int
    DocumentLimit   int
}

type DualResult struct {
    MemoryHits   []Hit
    DocumentHits []Hit
    Warnings     []string
}
```

Tests prove:

- embedder is called once for both sources
- both searchers receive the same vector
- two blocking fakes overlap, proving concurrent search
- one infrastructure failure returns the other hits plus one warning
- both infrastructure failures return two warnings and no hard error
- integrity failure is returned as a hard error
- embedding failure produces a warning and no search calls
- disabled sources are not called
- request validation requires ownership and positive enabled-source limits

- [x] **Step 4: Run RED**

```bash
go test ./internal/contextretrieval ./internal/memoryitem -run 'Test(Memory|Dual|QdrantData)'
```

Expected: FAIL because adapters, orchestration and typed memory data errors do not exist.

- [x] **Step 5: Implement memory adapter and dual retriever**

Use constructors:

```go
func NewMemoryQdrantSearcher(index MemoryVectorSearcher) *MemoryQdrantSearcher
func NewDualRetriever(
    embedder QueryEmbedder,
    embeddingModel string,
    memory MemorySearcher,
    documents DocumentSearcher,
) *DualRetriever

func (r *DualRetriever) Retrieve(ctx context.Context, req DualRequest) (DualResult, error)
```

The two goroutines write to separate local result variables and a buffered
two-result channel; no shared slice is mutated concurrently. Always wait for both
started searches before returning.

Error policy:

```go
if IsInfrastructureFailure(err) {
    result.Warnings = append(result.Warnings, err.Error())
} else {
    return DualResult{}, err
}
```

Memory adapter maps `memoryitem.IsQdrantDataError` to IntegrityFailure and all
other search errors to InfrastructureFailure.

- [x] **Step 6: Run GREEN and race tests**

```bash
go test ./internal/contextretrieval ./internal/memoryitem
go test -race ./internal/contextretrieval ./internal/memoryitem
```

Expected: PASS, including the concurrency test under race detection.

- [x] **Step 7: Add and run real dual retrieval demo**

The command reads Ollama and both Qdrant collections, refuses unexpected
collection names, embeds one query once and searches:

- the existing primary memory fixture user
- knowledge scope offline-rag-course

Run:

```bash
go run ./cmd/dual-retrieval-demo --config config/recent-chat.env
```

Expected output contains:

```text
Query embeddings: 1
Memory hits:
Document hits:
Retrieval warnings: 0
Cross-user memory present: false
Cross-scope document present: false
```

The command exits non-zero if warnings occur so standalone diagnosis is strict.

- [x] **Step 8: Write SOP and operation log**

Explain one shared query vector, concurrent I/O, source-specific ownership,
failure classification and why payload isolation failures are not degraded.

- [x] **Step 9: Review and verify**

```bash
gofmt -w internal/contextretrieval/*.go internal/memoryitem/*.go cmd/dual-retrieval-demo/*.go
go test ./...
go test -race ./internal/contextretrieval ./internal/memoryitem
go vet ./...
go build ./cmd/...
git diff --check
```

Review goroutine completion, cancellation, error wrapping, user/scope validation
and collection guards.

- [x] **Step 10: Commit lesson 26**

```bash
git add internal/contextretrieval internal/memoryitem cmd/dual-retrieval-demo docs/teaching/dual-retrieval-sop.md docs/teaching/00-dual-retrieval-batch-operation-log.md
git commit -m "feat: retrieve memory and documents in parallel"
```

---

### Task 27: Deterministic Merge, Safe Rendering and Exact Context Budget

**Files:**
- Create: `internal/contextretrieval/merge.go`
- Create: `internal/contextretrieval/merge_test.go`
- Create: `internal/contextretrieval/prompt.go`
- Create: `internal/contextretrieval/prompt_test.go`
- Create: `internal/contextretrieval/budget.go`
- Create: `internal/contextretrieval/budget_test.go`
- Create: `cmd/context-merge-demo/main.go`
- Create: `docs/teaching/context-merge-budget-sop.md`
- Modify: `docs/teaching/00-dual-retrieval-batch-operation-log.md`
- Modify: `docs/teaching/00-optimization-backlog.md`

**Interfaces:**
- Consumes: Task 26 DualResult and Task 24 validated Hit.
- Produces: `Merge`, `RenderContext`, `SelectWithinTokenBudget`, `ContextSelection`.

- [ ] **Step 1: Write deterministic merge RED tests**

Use:

```go
type MergeLimits struct {
    Memory   int
    Documents int
}

func Merge(memoryHits, documentHits []Hit, limits MergeLimits) ([]Hit, error)
```

Tests prove independent descending-score sort, stable ID tiebreak, separate
limits, memory-first output, input slices unchanged and normalized-content
deduplication across sources.

Normalization for deduplication is exactly:

```go
strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
```

If a document duplicates retained memory content, retain memory and drop the
document. Empty/invalid hits return errors.

- [ ] **Step 2: Write safe rendering RED tests**

Use:

```go
func RenderContext(hits []Hit) (string, error)
```

The block starts with a fixed instruction that retrieved content is untrusted
data, not instructions. Test that `<script>`, `</memory>` and ampersands in
content/title/source are HTML-escaped and cannot close structural tags.

Each memory element includes ID, kind and content. Each document includes ID,
title, source_ref and content.

- [ ] **Step 3: Write exact budget RED tests**

Use:

```go
type TextTokenCounter interface {
    CountText(text string) (count int, tokens []string, ids []int, err error)
}

type ContextSelection struct {
    Hits       []Hit
    Rendered   string
    UsedTokens int
    DroppedIDs []string
}

func SelectWithinTokenBudget(
    candidates []Hit,
    maxTokens int,
    counter TextTokenCounter,
) (ContextSelection, error)
```

Tests use a deterministic fake counter and prove:

- selected Rendered count never exceeds maxTokens
- an oversized candidate is skipped and later smaller candidates may fit
- dropped IDs preserve candidate order
- zero/negative budget and nil counter are rejected
- tokenizer errors propagate
- empty candidates produce an empty selection with zero tokens

- [ ] **Step 4: Run RED**

```bash
go test ./internal/contextretrieval -run 'Test(Merge|RenderContext|SelectWithin)'
```

Expected: FAIL because merge, renderer and budgeter do not exist.

- [ ] **Step 5: Implement merge, renderer and budgeter**

Do not mutate caller slices or Metadata maps. For each candidate:

1. append a copied candidate to the tentative selection
2. render the complete tentative block
3. count the rendered block once
4. keep it if count <= maxTokens
5. otherwise record its ID in DroppedIDs and continue

After selection, render and count the final block once more. If the final count
differs from the last retained tentative count, return an error because the
counter must be deterministic.

- [ ] **Step 6: Run GREEN**

```bash
go test ./internal/contextretrieval
go test -race ./internal/contextretrieval
```

Expected: PASS.

- [ ] **Step 7: Add and run the merge/budget practice**

The command loads the real tokenizer, uses fixed memory/document hits including
one duplicate and one oversized hit, then prints merge and budget observations.

Run:

```bash
go run ./cmd/context-merge-demo \
  --config config/recent-chat.env \
  --context-token-budget 160
```

Expected output contains:

```text
Memory candidates:
Document candidates:
Duplicate removed:
Selected source order:
Used context tokens:
Within budget: true
Rendered retrieved_context:
```

- [ ] **Step 8: Write SOP and production backlog**

Explain why raw scores from separate collections are not globally compared,
why quotas are deterministic, how dedupe precedence works, why complete block
rendering is counted and how prompt injection boundaries work.

Add deferred reranking, score calibration and dynamic quotas to the backlog.

- [ ] **Step 9: Review and verify**

```bash
gofmt -w internal/contextretrieval/*.go cmd/context-merge-demo/*.go
go test ./...
go test -race ./internal/contextretrieval
go vet ./...
go build ./cmd/...
git diff --check
```

Review deterministic ordering, non-mutation, escaping, exact token count and
oversized-hit behavior.

- [ ] **Step 10: Commit lesson 27**

```bash
git add internal/contextretrieval cmd/context-merge-demo docs/teaching/context-merge-budget-sop.md docs/teaching/00-dual-retrieval-batch-operation-log.md docs/teaching/00-optimization-backlog.md
git commit -m "feat: merge and budget retrieved context"
```

---

### Task 28: Inject Dual Retrieval into Real recent-chat

**Files:**
- Modify: `internal/recentchat/types.go`
- Modify: `internal/recentchat/http.go`
- Modify: `internal/recentchat/service.go`
- Create: `internal/recentchat/service_retrieval_test.go`
- Modify: `cmd/recent-chat/main.go`
- Create: `docs/teaching/recent-chat-dual-retrieval-sop.md`
- Modify: `docs/teaching/00-dual-retrieval-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-handoff-guide.md`
- Modify: `config/recent-chat.env.example`

**Interfaces:**
- Consumes: Task 26 DualRetriever and Task 27 merge/render/budget.
- Produces: optional retrieval request fields, ChatContext, response observations and real service wiring.

- [ ] **Step 1: Write request validation RED tests**

Add fields:

```go
UseMemory         bool   `json:"use_memory"`
UseKnowledge      bool   `json:"use_knowledge"`
KnowledgeScope    string `json:"knowledge_scope"`
MemoryLimit       int    `json:"memory_limit"`
DocumentLimit     int    `json:"document_limit"`
ContextTokenBudget int   `json:"context_token_budget"`
```

Validation rules:

- retrieval requires auto_token_budget
- enabled memory requires memory_limit > 0
- enabled knowledge requires non-empty knowledge_scope and document_limit > 0
- any enabled source requires context_token_budget > 0
- disabled old requests remain valid without the new fields

- [ ] **Step 2: Write service RED tests**

Add:

```go
type ContextRetriever interface {
    Retrieve(ctx context.Context, req contextretrieval.DualRequest) (contextretrieval.DualResult, error)
}

func NewServiceWithContextRetrieval(base Service, retriever ContextRetriever) Service
func (s Service) ChatContext(ctx context.Context, req ChatRequest) (ChatResponse, error)
```

Keep `Chat(req)` as a compatibility wrapper using `context.Background()`.

Tests prove:

- no retrieval call when flags are false
- exact user ID, knowledge scope and limits reach the retriever
- merged and budgeted context appears once in the Ollama system message
- automatic planner fixed input includes retrieved context
- response exposes selected hits, source counts, used tokens and warnings
- infrastructure warnings still call Ollama
- integrity error prevents Ollama and message writes
- another-user or another-scope hit is rejected before prompt construction
- request context cancellation reaches the retriever
- summary combines base system, retrieved block and summary exactly once
- context selection reducing fixed capacity reduces available recent tokens

- [ ] **Step 3: Run RED**

```bash
go test ./internal/recentchat -run 'Test(ChatRequestRetrieval|ServiceRetrieval)'
```

Expected: FAIL because request fields, interface and ChatContext do not exist.

- [ ] **Step 4: Implement request and response observations**

Add response fields:

```go
RetrievedContext      []contextretrieval.Hit `json:"retrieved_context"`
UsedMemoryItems       int                    `json:"used_memory_items"`
UsedDocumentChunks    int                    `json:"used_document_chunks"`
UsedContextTokens     int                    `json:"used_context_tokens"`
RetrievalWarnings     []string               `json:"retrieval_warnings"`
```

When retrieval is enabled:

1. call ContextRetriever before initial automatic budget planning
2. Merge with request limits
3. SelectWithinTokenBudget with the existing real token counter
4. combine rendered context with the original system prompt
5. use the combined system prompt in the existing automatic planner
6. continue summary and recent-window selection
7. place the final system message before recent history and current user
8. return observations without exposing vectors

Change the HTTP handler to call:

```go
resp, err := svc.ChatContext(r.Context(), req)
```

- [ ] **Step 5: Wire real clients in cmd/recent-chat**

Read existing config keys with defaults:

```text
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_EMBED_MODEL=bge-m3
QDRANT_BASE_URL=http://127.0.0.1:6333
QDRANT_MEMORY_COLLECTION=offline_rag_memory_items_v1
QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
```

Construct:

```go
embedder := memoryitem.NewHTTPOllamaEmbedder(ollamaBaseURL)
memoryIndex := memoryitem.NewQdrantIndexer(qdrantBaseURL, memoryCollection)
memorySearch := contextretrieval.NewMemoryQdrantSearcher(memoryIndex)
documentSearch := contextretrieval.NewDocumentQdrant(qdrantBaseURL, documentCollection)
dual := contextretrieval.NewDualRetriever(embedder, embeddingModel, memorySearch, documentSearch)
service = recentchat.NewServiceWithContextRetrieval(service, dual)
```

Startup must not create collections or write points. Collection creation remains
the explicit lesson 25 demo.

- [ ] **Step 6: Run GREEN and targeted race tests**

```bash
go test ./internal/recentchat ./internal/contextretrieval ./internal/memoryitem
go test -race ./internal/recentchat ./internal/contextretrieval ./internal/memoryitem
```

Expected: PASS.

- [ ] **Step 7: Start service and run real curl verification**

Start:

```bash
go run ./cmd/recent-chat --config config/recent-chat.env --addr :18093
```

Request:

```bash
curl -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"dual-retrieval-chat-001",
    "user_id":"memory-store-demo-user-20260712-a",
    "message":"这个项目使用什么语言，教学要求是什么？",
    "model":"qwen:7b",
    "recent_limit":10,
    "auto_token_budget":true,
    "output_token_reserve":512,
    "use_session_summary":false,
    "use_memory":true,
    "use_knowledge":true,
    "knowledge_scope":"offline-rag-course",
    "memory_limit":3,
    "document_limit":3,
    "context_token_budget":512,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

Verify the JSON contains non-zero memory/document counts, no cross-owner hits,
used_context_tokens <= 512, answer text and stored user/assistant turns.

Run a wrong-scope request and verify used_document_chunks is zero while chat still
returns an answer. Run with an unavailable document collection through a temporary
ignored config and verify a document warning while memory and chat still work.

- [ ] **Step 8: Write final SOP and update handoff state**

The SOP explains the full order:

```text
validate
-> retrieve and classify failures
-> merge
-> exact context budget
-> automatic fixed-input plan
-> summary
-> recent window
-> Ollama
-> MySQL message writes
```

Update learning status to say lessons 24-28 are implemented and verified but not
yet user-confirmed as learned. Set the next implementation chapter to production
chunking/document ingestion and retrieval evaluation.

Update handoff minimum-reading list with the five new SOPs.

- [ ] **Step 9: Final review and complete verification**

```bash
gofmt -w internal/contextretrieval/*.go internal/memoryitem/*.go internal/recentchat/*.go cmd/recent-chat/*.go
go test ./...
go test -race ./internal/contextretrieval ./internal/memoryitem ./internal/recentchat
go vet ./...
go build ./cmd/...
git diff --check
```

Also inspect:

```bash
git status --short
git diff --check HEAD
git log --oneline -8
```

Confirm no real DSN, credential, vector payload dump, full machine path or old
collection mutation appears in tracked changes.

- [ ] **Step 10: Commit lesson 28**

```bash
git add internal/contextretrieval internal/recentchat cmd/recent-chat config/recent-chat.env.example docs/teaching/recent-chat-dual-retrieval-sop.md docs/teaching/00-dual-retrieval-batch-operation-log.md docs/teaching/00-learning-status.md docs/teaching/00-handoff-guide.md
git commit -m "feat: inject dual retrieval into recent chat"
```

## Completion Audit

Before marking the goal complete, verify authoritative evidence for every goal item:

- exactly five lesson implementation commits exist after the plan commit
- every lesson has Go code, comments, tests, runnable practice, SOP and review evidence
- offline_rag_document_chunks_v1 exists with 1024/Cosine and required payload indexes
- real dual retrieval proves user and knowledge-scope isolation
- real recent-chat curl proves memory, documents, recent flow and Ollama answer coexist
- learning status says implemented/verified, not learned
- AI_INITIALIZATION.md remains committed
- no push occurred
- all final test, race, vet, build, format and diff checks pass
