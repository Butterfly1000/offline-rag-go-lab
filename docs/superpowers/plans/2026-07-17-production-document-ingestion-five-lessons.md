# Production Document Ingestion Five Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and teach a real local pipeline that versions Markdown/Go documents in MySQL, creates stable tokenizer-bounded chunks, indexes them in isolated Qdrant snapshots, switches a stable alias safely, and evaluates retrieval with at least ten golden cases.

**Architecture:** `internal/documentingest` owns identities, parsers, state transitions, ingestion orchestration, Qdrant publication and evaluation. MySQL is the authoritative manifest; Qdrant is a rebuildable derived index; Ollama supplies local `bge-m3` embeddings. Existing document and memory collections are not modified.

**Tech Stack:** Go 1.23 module semantics, standard library `go/parser`, local qwen tokenizer through `internal/tokenizerdemo`, MySQL 8 through `database/sql`, Qdrant HTTP API, Ollama `/api/embed`, JSON fixtures and reports.

## Global Constraints

- Work in `/offline-rag-go-lab` on the current teaching commit sequence; commit but never push.
- Use only local MySQL `127.0.0.1:3306/offline_rag`, Qdrant `127.0.0.1:6333`, Ollama `127.0.0.1:11434`, and repository files.
- Read runtime values from `config/recent-chat.env`; do not require shell environment variables or commit credentials.
- Never write `offline_rag_document_chunks_v1`, `offline_rag_memory_items_v1`, or `ollama_chat_memory`.
- Use only `offline_rag_document_ingestion_lab_v1`, `_v2`, and alias `offline_rag_document_ingestion_lab_active`.
- Do not delete physical collections; rollback only switches the alias.
- Every lesson follows RED -> GREEN -> practice -> SOP -> review -> independent commit.
- Implementation completion must remain distinct from user-confirmed learning completion.

---

### Task 1: Lesson 29 - Document Identity and Version State

**Files:**
- Create: `internal/documentingest/types.go`
- Create: `internal/documentingest/identity.go`
- Create: `internal/documentingest/identity_test.go`
- Create: `internal/documentingest/state.go`
- Create: `internal/documentingest/state_test.go`
- Create: `cmd/document-identity-demo/main.go`
- Create: `sql/document_ingestion.sql`
- Create: `docs/teaching/document-identity-version-sop.md`
- Create: `docs/teaching/00-document-ingestion-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`

**Interfaces:**
- Produces: `Document`, `ChunkIdentityInput`, `VersionStatus`, `ContentHash`, `ChunkID`, `ChunkPolicyHash`, and `ValidateTransition`.
- Identity signatures:

```go
func NormalizeDocument(input Document) (Document, error)
func ContentHash(content []byte) string
func ChunkPolicyHash(policy ChunkPolicyIdentity) (string, error)
func StableChunkID(input ChunkIdentityInput) (string, error)
func ValidateTransition(from, to VersionStatus) error
```

- `StableChunkID` hashes scope, document ID, structure kind, heading path, normalized content hash and duplicate ordinal, separated by NUL bytes.
- `source_ref` must be slash-separated, relative, clean, and unable to escape with `..`.

- [ ] **Step 1: Write failing identity and state tests**

Cover unchanged chunks across versions, changed text, moved heading path, inserted unrelated earlier content, repeated identical blocks, whitespace normalization, invalid identity values, every legal transition and illegal active mutation.

```go
func TestStableChunkIDSurvivesDocumentVersionChange(t *testing.T) {
    first, err := StableChunkID(validChunkIdentity("same text"))
    if err != nil { t.Fatal(err) }
    second, err := StableChunkID(validChunkIdentity("same text"))
    if err != nil { t.Fatal(err) }
    if first != second { t.Fatalf("IDs differ: %q != %q", first, second) }
}
```

- [ ] **Step 2: Run RED and record the expected failure**

Run:

```bash
go test ./internal/documentingest -run 'Test(StableChunkID|NormalizeDocument|ContentHash|ChunkPolicyHash|ValidateTransition)'
```

Expected: package build fails because the new identity and state APIs do not exist.

- [ ] **Step 3: Implement the minimal domain model and SQL schema**

Use SHA256 lowercase hex, explicit normalization and a closed transition table. Define the three InnoDB tables with `utf8mb4`, version/chunk foreign keys, unique build identity, unique per-version chunk identity and indexes for source/status lookup. Keep `active_version_id` nullable; the activation transaction must verify that the referenced version belongs to the same source instead of introducing a circular foreign key that cannot be applied idempotently with plain `CREATE TABLE IF NOT EXISTS`.

- [ ] **Step 4: Run GREEN and race tests**

Run:

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

Expected: PASS with no local service access.

- [ ] **Step 5: Add the identity demo and teaching SOP**

`go run ./cmd/document-identity-demo` must print equal IDs for unchanged chunks and different IDs for changed/moved/duplicate chunks. The SOP explains logical document identity, immutable versions, why version is excluded from `chunk_id`, SQL constraints, current boundary and production consequences.

- [ ] **Step 6: Review and commit lesson 29**

Run gofmt, focused tests, `go vet ./internal/documentingest ./cmd/document-identity-demo`, `go build ./cmd/document-identity-demo`, `git diff --check`, inspect staged diff for credentials/full home paths, then commit:

```bash
git commit -m "feat: define production document identities"
```

---

### Task 2: Lesson 30 - Markdown and Go Structure-Aware Chunking

**Files:**
- Create: `internal/documentingest/chunker.go`
- Create: `internal/documentingest/chunker_test.go`
- Create: `internal/documentingest/markdown.go`
- Create: `internal/documentingest/markdown_test.go`
- Create: `internal/documentingest/golang.go`
- Create: `internal/documentingest/golang_test.go`
- Create: `internal/documentingest/token_counter.go`
- Create: `internal/documentingest/testdata/course.md`
- Create: `internal/documentingest/testdata/service.go.txt`
- Create: `cmd/document-chunk-demo/main.go`
- Create: `docs/teaching/structured-document-chunking-sop.md`
- Modify: `docs/teaching/00-document-ingestion-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`

**Interfaces:**

```go
type TokenCounter interface { Count(text string) (int, error) }
type ChunkPolicy struct { MaxTokens, OverlapLines int }
type Chunk struct {
    ChunkID, StructureKind, HeadingPath, Text, ContentHash string
    Ordinal, TokenCount int
}
func ChunkDocument(doc Document, policy ChunkPolicy, counter TokenCounter) ([]Chunk, error)
func NewQwenTokenCounter(path string) (TokenCounter, error)
```

- `Document.Format` accepts only `markdown` and `go` in this chapter.
- The parser emits structural units; one shared pack/split stage enforces the exact token ceiling and stable duplicate ordinals.

- [ ] **Step 1: Write failing parser and policy tests**

Cover Markdown heading stacks, paragraphs, fenced code with language marker, unclosed fences, Go declaration/doc-comment boundaries, malformed Go, oversized paragraph fallback, oversized code/declaration line splitting, overlap, deterministic order and every output at or below `MaxTokens`.

- [ ] **Step 2: Run RED**

```bash
go test ./internal/documentingest -run 'Test(Markdown|GoSource|ChunkDocument|SplitOversized)'
```

Expected: compile failure for missing parser/chunker APIs.

- [ ] **Step 3: Implement structural parsing and exact token enforcement**

Use a Markdown line-state scanner and standard-library `go/parser`. Do not classify arbitrary lines as headings. Use token counts at every packing decision; sentence/line/text fallback must make progress or return a bounded error instead of looping.

- [ ] **Step 4: Run GREEN and race tests**

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

Expected: PASS, including real tokenizer adapter tests when the checked-in asset is present.

- [ ] **Step 5: Add fixtures, demo and SOP**

Practice:

```bash
go run ./cmd/document-chunk-demo --config config/recent-chat.env --format markdown --source internal/documentingest/testdata/course.md --max-tokens 160
go run ./cmd/document-chunk-demo --config config/recent-chat.env --format go --source internal/documentingest/testdata/service.go.txt --max-tokens 160
```

Print structure path, kind, token count, chunk ID and a bounded preview. The SOP explains observed output, exact token enforcement, why overlap is restricted to oversized units, failure behavior and deferred formats.

- [ ] **Step 6: Review and commit lesson 30**

Run gofmt, focused/race tests, vet/build, demo commands and diff checks. Review for dropped source text, infinite fallback loops, nondeterministic map iteration and accidental character-count limits. Commit:

```bash
git commit -m "feat: chunk markdown and go by structure"
```

---

### Task 3: Lesson 31 - Real Idempotent MySQL and Qdrant Ingestion

**Files:**
- Create: `internal/documentingest/store.go`
- Create: `internal/documentingest/store_mysql.go`
- Create: `internal/documentingest/store_mysql_test.go`
- Create: `internal/documentingest/qdrant.go`
- Create: `internal/documentingest/qdrant_test.go`
- Create: `internal/documentingest/ingest.go`
- Create: `internal/documentingest/ingest_test.go`
- Create: `cmd/document-ingest-demo/main.go`
- Create: `docs/teaching/idempotent-document-ingestion-sop.md`
- Modify: `config/recent-chat.env.example`
- Modify: `docs/teaching/00-document-ingestion-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`

**Interfaces:**

```go
type ManifestStore interface {
    FindOrCreateVersion(ctx context.Context, build BuildIdentity) (Version, error)
    ClaimBuild(ctx context.Context, versionID int64) error
    SaveReadyManifest(ctx context.Context, versionID int64, chunks []ChunkManifest) error
    MarkFailed(ctx context.Context, versionID int64, reason string) error
}
type VectorIndex interface {
    EnsureCollection(ctx context.Context, name string, vectorSize int) error
    UpsertBatch(ctx context.Context, name string, points []VectorPoint) error
    DeletePoints(ctx context.Context, name string, pointIDs []string) error
}
type BatchEmbedder interface { Embed(ctx context.Context, model string, texts []string) ([][]float32, error) }
func (s *IngestionService) Ingest(ctx context.Context, request IngestRequest) (IngestResult, error)
```

- All mutable MySQL transitions use transactions and expected-state predicates.
- Qdrant requests use bounded batches, `wait=true`, a bounded HTTP client and exact dimension validation.
- Same ready/active build returns `Noop=true` before embedding or Qdrant writes.

- [ ] **Step 1: Write failing store, Qdrant and orchestration tests**

Use fake SQL/query adapters and `httptest`. Prove no-op skips embedding, partial retry reuses IDs, dimension mismatch fails before ready, manifest save happens after all upserts, errors mark failed, cancellation propagates, batches are bounded, and delete requires explicit non-empty IDs/collection.

- [ ] **Step 2: Run RED**

```bash
go test ./internal/documentingest -run 'Test(MySQLManifest|QdrantIndex|Ingestion)'
```

Expected: compile failure for missing persistence and ingestion APIs.

- [ ] **Step 3: Implement MySQL, Qdrant and ingestion service**

Reuse `memoryitem.HTTPOllamaEmbedder` behind the narrow embedder interface. Keep all Qdrant collection names caller-supplied and validate the approved prefix in the executable before any mutation. Bound saved errors to 2048 bytes.

- [ ] **Step 4: Run GREEN and package race tests**

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

Expected: PASS using only fakes/`httptest`.

- [ ] **Step 5: Add real local command and run the idempotency SOP**

Add example keys:

```text
DOCUMENT_INGEST_SCHEMA_PATH=sql/document_ingestion.sql
DOCUMENT_INGEST_COLLECTION_V1=offline_rag_document_ingestion_lab_v1
DOCUMENT_INGEST_COLLECTION_V2=offline_rag_document_ingestion_lab_v2
DOCUMENT_INGEST_ALIAS=offline_rag_document_ingestion_lab_active
```

Practice sequence:

```bash
go run ./cmd/document-ingest-demo --config config/recent-chat.env --apply-schema --ensure-collection --collection offline_rag_document_ingestion_lab_v1 --scope document-ingestion-course --document-id course-markdown --format markdown --source internal/documentingest/testdata/course.md
go run ./cmd/document-ingest-demo --config config/recent-chat.env --collection offline_rag_document_ingestion_lab_v1 --scope document-ingestion-course --document-id course-markdown --format markdown --source internal/documentingest/testdata/course.md
```

Capture table/point counts before and after the second run. The second result must be `noop`, with zero embed/upsert batches and unchanged counts.

- [ ] **Step 6: Review and commit lesson 31**

Review transaction boundaries, SQL uniqueness, collection guard, retries, context cancellation, body/error limits, credentials and evidence from real local runs. Run focused/race tests, vet/build and diff checks. Commit:

```bash
git commit -m "feat: ingest document versions idempotently"
```

---

### Task 4: Lesson 32 - Verified Snapshot Rebuild, Alias Switch and Rollback

**Files:**
- Create: `internal/documentingest/publish.go`
- Create: `internal/documentingest/publish_test.go`
- Create: `internal/documentingest/qdrant_alias.go`
- Create: `internal/documentingest/qdrant_alias_test.go`
- Create: `cmd/document-publish-demo/main.go`
- Create: `docs/teaching/document-snapshot-alias-sop.md`
- Modify: `docs/teaching/00-document-ingestion-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`

**Interfaces:**

```go
type SnapshotIndex interface {
    CollectionInfo(ctx context.Context, name string) (CollectionInfo, error)
    Count(ctx context.Context, name, scope string) (int, error)
    Fetch(ctx context.Context, name string, pointIDs []string) ([]IndexedPoint, error)
    ResolveAlias(ctx context.Context, alias string) (string, error)
    SwitchAlias(ctx context.Context, alias, from, to string) error
}
func (p *Publisher) Verify(ctx context.Context, request VerifyRequest) (VerificationReport, error)
func (p *Publisher) Activate(ctx context.Context, request ActivateRequest) (ActivationResult, error)
func (p *Publisher) Rollback(ctx context.Context, request RollbackRequest) error
```

- Activation requires a current verification report tied to target collection plus expected manifest digest.
- Alias switch is one Qdrant `/collections/aliases` request containing delete-old/add-new actions.
- MySQL activation after alias success is explicit reconciliation, not a false distributed transaction.

- [ ] **Step 1: Write failing verification/alias tests**

Prove vector config, required indexes, count, fetched ID/hash/scope, stale verification, unexpected current alias, atomic action body, MySQL activation failure reconciliation result and rollback action order.

- [ ] **Step 2: Run RED**

```bash
go test ./internal/documentingest -run 'Test(Publisher|QdrantAlias|VerifySnapshot)'
```

Expected: compile failure for missing publication APIs.

- [ ] **Step 3: Implement verification, switch and rollback**

Never delete a collection. Refuse targets outside the lab prefix and refuse alias activation when verification differs from the current manifest digest.

- [ ] **Step 4: Run GREEN and race tests**

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

- [ ] **Step 5: Run the real local rebuild SOP**

Ingest the complete fixture corpus into `_v1`, verify and point the alias to `_v1`; ingest an updated complete corpus into `_v2`, verify and switch to `_v2`; query/resolve the alias; roll back to `_v1`; switch to `_v2` again for the evaluation baseline. Record that both physical collections still exist.

- [ ] **Step 6: Review and commit lesson 32**

Review TOCTOU guards, exact alias request, no deletes, reconciliation output, scope validation and rollback evidence. Run focused/race tests, vet/build and diff checks. Commit:

```bash
git commit -m "feat: publish verified document snapshots"
```

---

### Task 5: Lesson 33 - Golden Retrieval Evaluation

**Files:**
- Create: `internal/documentingest/evaluate.go`
- Create: `internal/documentingest/evaluate_test.go`
- Create: `internal/documentingest/testdata/golden_queries.json`
- Create: `internal/documentingest/testdata/course_competing.md`
- Create: `cmd/document-eval-demo/main.go`
- Create: `docs/teaching/document-retrieval-evaluation-sop.md`
- Modify: `docs/teaching/00-document-ingestion-batch-operation-log.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-optimization-backlog.md`

**Interfaces:**

```go
type GoldenCase struct {
    CaseID, Query, KnowledgeScope, Notes string
    ExpectedChunkIDs, ForbiddenChunkIDs []string
}
type CaseResult struct { RecallAt3, MRRAt3 float64; ScopeIsolated bool; ForbiddenHits []string }
type EvaluationReport struct { CaseCount int; MeanRecallAt3, MeanMRRAt3 float64; ScopeIsolation float64; Passed bool; Cases []CaseResult }
func Evaluate(ctx context.Context, cases []GoldenCase, embedder QueryEmbedder, searcher ScopedSearcher) (EvaluationReport, error)
```

- Search limit is exactly three.
- Each case must have one to three unique expected IDs and at least one forbidden ID from a competing scope or topic.
- Stable JSON output sorts cases by `case_id` and IDs lexically where rank is not meaningful.

- [ ] **Step 1: Write failing metric and isolation tests**

Cover full/partial/missing recall, first/second/third-rank MRR, duplicate result IDs, cross-scope hard failure, forbidden hits, fewer than ten cases, invalid expected sets, deterministic report order and one embedding call per case.

- [ ] **Step 2: Run RED**

```bash
go test ./internal/documentingest -run 'Test(Evaluate|RecallAt3|MRRAt3|Golden)'
```

Expected: compile failure for missing evaluator APIs.

- [ ] **Step 3: Implement evaluator and reviewed dataset**

Use at least ten paraphrased questions covering Markdown and Go fixtures plus competing-scope negatives. Never infer scope from returned text; use and revalidate payload ownership.

- [ ] **Step 4: Run GREEN and race tests**

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

- [ ] **Step 5: Run real evaluation and save observed evidence in the SOP**

```bash
go run ./cmd/document-eval-demo --config config/recent-chat.env --alias offline_rag_document_ingestion_lab_active --golden internal/documentingest/testdata/golden_queries.json --k 3
```

Required result: at least ten cases, every case Recall@3=1.0, scope isolation=100%, forbidden hits=0 and process exit zero. Print aggregate MRR@3 without claiming it generalizes beyond the fixture corpus.

- [ ] **Step 6: Review and commit lesson 33**

Review for metric denominator/rank errors, fixture leakage, decorative cases, missing competing scopes, unstable JSON and mismatch between SOP/output. Run focused/race tests, vet/build and diff checks. Commit:

```bash
git commit -m "feat: evaluate scoped document retrieval"
```

---

### Task 6: Final Completion Audit

**Files:**
- Modify only if evidence exposes a real gap: files from Tasks 1-5.

- [ ] **Step 1: Prove all objective artifacts exist**

Check five SOPs, five lesson commits, RED/GREEN evidence in the operation log, at least ten golden cases, local runtime evidence and learning status marked implemented-but-not-learned.

- [ ] **Step 2: Run final gates from fresh output**

```bash
go test ./...
go test -race ./internal/documentingest ./internal/contextretrieval
go vet ./...
go build ./...
git diff --check
git status --short --branch
```

Expected: all commands pass and worktree is clean. Inspect changed Go files with `gofmt -d`; expected output is empty.

- [ ] **Step 3: Audit local external state without deleting anything**

Verify three MySQL tables, `_v1` and `_v2` collections, stable alias target, expected point/manifest counts, 1024/Cosine configuration, required payload indexes and no changes to protected collections.

- [ ] **Step 4: Complete the goal without push**

Summarize commits and verification evidence. Mark the goal complete only if every objective requirement has authoritative evidence. Do not run `git push`.
