# Retrieval Quality Five Lessons Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and verify L34-L38 as a reproducible local retrieval-quality pipeline from a versioned golden dataset through calibrated dynamic policy.

**Architecture:** Add a focused `internal/retrievalquality` package with pure domain components and narrow Ollama/Qdrant adapters. Keep L33 unchanged, build a new isolated named-vector Qdrant snapshot, and make every later strategy consume the fixed L34 dataset and baseline.

**Tech Stack:** Go 1.23, standard-library HTTP/JSON, existing `sugarme/tokenizer`, local Ollama `bge-m3` and `qwen:7b`, local Qdrant Query API.

## Global Constraints

- Work directly on the current branch; do not create or switch branches.
- Do not push.
- Write only `offline_rag_retrieval_quality_lab_v1` and alias `offline_rag_retrieval_quality_lab_active`.
- Never delete a physical Qdrant collection or modify existing Memory, recent-chat, L25, or L31-L33 collections.
- Keep `internal/documentingest` L33 behavior intact.
- Every production behavior starts with an observed, correctly failing RED test.
- Every lesson ends with focused tests, a real local command, an SOP, review, and an independent commit.
- Only stage files named by this plan; leave the three unrelated untracked AirDroid documents untouched.
- Strategy tuning reads only the 24 train cases; the 16 validation cases are report-only.

---

## File Map

Create these focused package files:

- `dataset.go`: dataset types, disk loader, checksums, and contract validation
- `metrics.go`: Recall/MRR/NDCG/negative/isolation/latency calculations
- `evaluate.go`: deterministic strategy evaluation and failure classification
- `dense_memory.go`: L34 real-Ollama in-memory cosine baseline
- `sparse.go`: tokenizer adapter, field statistics, BM25-style sparse encoder
- `qdrant.go`: named-vector schema, upsert, alias, dense/sparse query, payload validation
- `fusion.go`: weighted RRF and trace
- `hybrid.go`: two-leg execution, hard/soft error boundary, deterministic fallback
- `rerank.go`: candidate validation, rerank ordering, and fallback
- `rerank_ollama.go`: strict local Ollama JSON adapter
- `diversity.go`: document/heading caps and skip reasons
- `calibration.go`: PAVA isotonic calibration, Brier, and ECE
- `policy.go`: query classification, train-only parameter selection, artifact identity
- `regression.go`: final validation regression gates

Create one command split into subcommand files under `cmd/retrieval-quality-demo`.
Shared flags load the ignored project config; each subcommand emits stable JSON.

## Task 1: L34 Versioned Production Golden Dataset

**Files:**

- Create: `internal/retrievalquality/dataset.go`
- Create: `internal/retrievalquality/dataset_test.go`
- Create: `internal/retrievalquality/metrics.go`
- Create: `internal/retrievalquality/metrics_test.go`
- Create: `internal/retrievalquality/evaluate.go`
- Create: `internal/retrievalquality/evaluate_test.go`
- Create: `internal/retrievalquality/dense_memory.go`
- Create: `internal/retrievalquality/dense_memory_test.go`
- Create: `internal/retrievalquality/testdata/golden/v1/manifest.json`
- Create: `internal/retrievalquality/testdata/golden/v1/corpus.json`
- Create: `internal/retrievalquality/testdata/golden/v1/cases.json`
- Create: `cmd/retrieval-quality-demo/main.go`
- Create: `cmd/retrieval-quality-demo/dataset.go`
- Create: `docs/teaching/production-golden-dataset-sop.md`
- Create: `docs/teaching/00-retrieval-quality-batch-operation-log.md`

**Interfaces:**

- Produces:
  - `LoadDataset(dir string) (Dataset, error)`
  - `Dataset.Validate() error`
  - `Evaluate(ctx context.Context, dataset Dataset, split Split, strategy Strategy, k int) (Report, error)`
  - `NewDenseMemoryStrategy(embedder Embedder, model string, corpus []Chunk) (*DenseMemoryStrategy, error)`

- [ ] **Step 1: Write loader and contract RED tests**

Write table tests whose valid fixture expects exactly 40 cases, 24 train, 16 validation,
five query kinds, and two scopes. Mutation cases must fail for checksum mismatch,
duplicate IDs, missing referenced chunks, cross-scope judgment, overlapping
relevant/forbidden IDs, an invalid relevance grade, and the wrong split counts.

```go
func TestLoadDatasetV1(t *testing.T) {
    dataset, err := LoadDataset("testdata/golden/v1")
    if err != nil { t.Fatal(err) }
    if got := len(dataset.Cases); got != 40 { t.Fatalf("cases=%d", got) }
    if got := dataset.CountSplit(SplitTrain); got != 24 { t.Fatalf("train=%d", got) }
    if got := dataset.CountSplit(SplitValidation); got != 16 { t.Fatalf("validation=%d", got) }
}
```

- [ ] **Step 2: Run the loader tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestLoadDataset|TestDataset'`.
Expected failure: package or `LoadDataset` is missing, not fixture syntax failure.

- [ ] **Step 3: Implement the immutable dataset contract**

Use raw-file SHA256 for `corpus.json` and `cases.json`. Normalize only values used
for validation; do not silently rewrite checked-in data. Define exact enums:

```go
type Split string
const (SplitTrain Split = "train"; SplitValidation Split = "validation")
type QueryKind string
const (
    QueryExact QueryKind = "exact"; QueryCode QueryKind = "code"
    QuerySemantic QueryKind = "semantic"; QueryMixed QueryKind = "mixed"
    QueryNegative QueryKind = "negative"
)
```

Reject any dataset that does not satisfy the v1 counts and coverage in the design.

- [ ] **Step 4: Write metrics/evaluator RED tests**

Use a fake strategy with graded results to prove Recall@3/10, MRR@10, NDCG@10,
negative pass, forbidden hits, hard cross-scope rejection, p50/p95, category
grouping, stable case ordering, and failure stage.

```go
report, err := Evaluate(ctx, dataset, SplitValidation, strategy, 10)
if err != nil { t.Fatal(err) }
if report.ForbiddenHitCount != 0 || report.ScopeIsolation != 1 {
    t.Fatalf("unsafe report: %+v", report)
}
```

- [ ] **Step 5: Run evaluator tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestEvaluate|TestNDCG|TestPercentile'`.
Expected failure: missing evaluator/metric symbols.

- [ ] **Step 6: Implement metrics, evaluation, and the Dense in-memory strategy**

`Strategy.Search` returns candidates, an abstained bit, measured duration, trace,
and warnings. Dense baseline embeds corpus once, validates 1024 finite values,
embeds each query once, computes cosine, filters scope before scoring, and sorts
by score descending then chunk ID.

```go
type Strategy interface {
    Name() string
    Search(context.Context, Query) (SearchResult, error)
}
```

- [ ] **Step 7: Add the reviewed v1 corpus and 40 cases**

Use portable, synthetic course/operations material. For every content hash compute
SHA256 of exact content. For every case choose reviewed graded positives and a
forbidden chunk in the other scope. Compute raw corpus/case file checksums and
place them in `manifest.json`; do not weaken validation to fit the fixture.

- [ ] **Step 8: Implement and run the real L34 command**

`dataset` loads config, validates v1, embeds via local `bge-m3`, evaluates all/train/
validation, and prints model identity, dataset checksums, metrics, per-kind report,
and latency. Run:

```bash
go run ./cmd/retrieval-quality-demo dataset \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1
```

Require 40 executed cases, scope isolation 1.0, and zero forbidden hits. Record
the complete command and observed aggregate metrics in both L34 SOP and batch log.

- [ ] **Step 9: Verify and commit L34**

Run `go test ./internal/retrievalquality ./cmd/retrieval-quality-demo`,
`gofmt`, `git diff --check`, and confirm only planned files are staged.
Commit: `feat: add production retrieval golden dataset`.

## Task 2: L35 Field-aware Sparse Retrieval

**Files:**

- Create: `internal/retrievalquality/sparse.go`
- Create: `internal/retrievalquality/sparse_test.go`
- Create: `internal/retrievalquality/qdrant.go`
- Create: `internal/retrievalquality/qdrant_test.go`
- Create: `cmd/retrieval-quality-demo/index.go`
- Create: `cmd/retrieval-quality-demo/sparse.go`
- Create: `docs/teaching/field-aware-sparse-retrieval-sop.md`
- Modify: `docs/teaching/00-retrieval-quality-batch-operation-log.md`

**Interfaces:**

- Consumes: `Dataset`, `Chunk`, `Query`, `Strategy`, `Report`
- Produces:
  - `BuildFieldStats(chunks []Chunk, tokenizer Tokenizer) (FieldStats, error)`
  - `NewSparseEncoder(tokenizer Tokenizer, stats FieldStats) (*SparseEncoder, error)`
  - `EncodeChunk(Chunk) (SparseVector, error)`
  - `EncodeQuery(string) (SparseVector, error)`
  - `NewQdrant(baseURL, physicalCollection, alias string) (*Qdrant, error)`
  - `EnsureCollection(context.Context, int) error`
  - `Upsert(context.Context, IndexedChunk) error`
  - `Activate(context.Context) error`
  - `DenseStrategy()` and `SparseStrategy()`

- [ ] **Step 1: Write sparse encoder RED tests**

Provide a fake tokenizer returning stable token IDs. Prove title=3.0,
heading=2.0, source=1.5, body=1.0 ordering; repeated terms receive BM25 saturation;
long fields receive length normalization; output indices are unique/sorted and
values finite/positive; empty or negative token IDs fail.

- [ ] **Step 2: Run sparse tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestSparse|TestBuildFieldStats'`.
Expected failure: sparse types/functions are missing.

- [ ] **Step 3: Implement field-aware sparse encoding**

Adapt `tokenizerdemo.Counter.CountText` behind:

```go
type Tokenizer interface { TokenIDs(string) ([]uint32, error) }
type SparseVector struct { Indices []uint32 `json:"indices"`; Values []float32 `json:"values"` }
```

Apply fixed weights and `k1=1.2`, `b=0.75`; merge duplicate indices after field
weighting and sort numerically.

- [ ] **Step 4: Write Qdrant HTTP RED tests**

Use `httptest` to assert exact named-vector create JSON, sparse IDF modifier,
keyword indexes, named-vector upsert, dense/sparse `using` query, mandatory scope
filter, stable point ID, alias activation, and payload identity/content/encoder
revalidation. Return a mismatched scope and prove it is a hard integrity error.

- [ ] **Step 5: Run Qdrant tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestQdrant'`.
Expected failure: Qdrant adapter is missing.

- [ ] **Step 6: Implement the narrow Qdrant adapter**

Use only standard-library HTTP. Create:

```json
{
  "vectors": {"dense": {"size": 1024, "distance": "Cosine"}},
  "sparse_vectors": {"sparse": {"modifier": "idf"}}
}
```

Validate existing schema instead of recreating it. Upsert named dense/sparse
vectors and payload identity. `Activate` may only alias the exact fixed physical
collection; no delete API exists in this adapter.

- [ ] **Step 7: Implement and run real index/sparse commands**

`index --apply --activate` embeds the fixed corpus, computes sparse vectors,
ensures collection/indexes, upserts deterministic points, verifies point count
and samples, then activates the alias. `sparse` evaluates both splits through the
alias. Run both commands and query Qdrant collection/alias metadata afterward.

- [ ] **Step 8: Document, verify, and commit L35**

Record exact/code and aggregate Sparse-vs-Dense differences, scope and forbidden
gates, collection identity, tokenizer checksum, and observed latency.
Run focused tests, `gofmt`, and `git diff --check`.
Commit: `feat: add field aware sparse retrieval`.

## Task 3: L36 Explainable Hybrid RRF

**Files:**

- Create: `internal/retrievalquality/fusion.go`
- Create: `internal/retrievalquality/fusion_test.go`
- Create: `internal/retrievalquality/hybrid.go`
- Create: `internal/retrievalquality/hybrid_test.go`
- Create: `cmd/retrieval-quality-demo/hybrid.go`
- Create: `docs/teaching/hybrid-retrieval-fusion-sop.md`
- Modify: `docs/teaching/00-retrieval-quality-batch-operation-log.md`

**Interfaces:**

- Consumes: Qdrant Dense/Sparse strategies and L34 evaluator
- Produces:
  - `FuseRRF(dense, sparse []Candidate, weights FusionWeights, rankConstant int) ([]Candidate, error)`
  - `NewHybrid(dense, sparse Strategy, weights FusionWeights) (*HybridStrategy, error)`

- [ ] **Step 1: Write weighted RRF RED tests**

Prove contribution formula, stable identity merge, absent-leg contribution zero,
tie break by best rank then chunk ID, duplicate/invalid candidates rejected, and
raw Dense/Sparse scores never affect fused ordering except through ranks.

- [ ] **Step 2: Run fusion tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestFuseRRF'`.
Expected failure: `FuseRRF` is missing.

- [ ] **Step 3: Implement weighted RRF and trace**

Each fused candidate contains leg rank/raw score/contribution and a deterministic
reason string. Validate positive finite weights and rank constant 60.

- [ ] **Step 4: Write hybrid error-boundary RED tests**

Prove both legs receive the same scope, evaluation mode fails if either leg is
unavailable, runtime mode degrades Sparse infrastructure failure to Dense with a
warning, and integrity/ownership failures remain hard errors.

- [ ] **Step 5: Implement hybrid orchestration**

Run legs independently with top 20, preserve measured leg latency, fuse only
validated identities, and expose `dense_only`, `sparse_only`, or `hybrid_rrf`
decision reasons.

- [ ] **Step 6: Run real Hybrid comparison**

Run `hybrid` twice and compare normalized JSON after removing measured durations;
rank order and reasons must match. Evaluate Dense, Sparse, and Hybrid on the same
dataset checksum and record per-kind quality plus p50/p95.

- [ ] **Step 7: Document, verify, and commit L36**

Run focused tests, `go test ./internal/retrievalquality`, `gofmt`, and
`git diff --check`. Commit: `feat: add explainable hybrid retrieval`.

## Task 4: L37 Local Reranker and Diversity

**Files:**

- Create: `internal/retrievalquality/rerank.go`
- Create: `internal/retrievalquality/rerank_test.go`
- Create: `internal/retrievalquality/rerank_ollama.go`
- Create: `internal/retrievalquality/rerank_ollama_test.go`
- Create: `internal/retrievalquality/diversity.go`
- Create: `internal/retrievalquality/diversity_test.go`
- Create: `cmd/retrieval-quality-demo/rerank.go`
- Create: `docs/teaching/reranker-diversity-sop.md`
- Modify: `docs/teaching/00-retrieval-quality-batch-operation-log.md`

**Interfaces:**

- Consumes: Hybrid candidates and evaluator
- Produces:
  - `Reranker.Rank(context.Context, string, []Candidate) ([]RerankScore, error)`
  - `NewOllamaReranker(baseURL, model string) (*OllamaReranker, error)`
  - `ApplyReranker(context.Context, Reranker, Query, []Candidate) RerankResult`
  - `ApplyDiversity([]Candidate, DiversityLimits) (kept []Candidate, skipped []DiversitySkip, err error)`

- [ ] **Step 1: Write reranker protocol RED tests**

Prove exact candidate coverage, no unknown/duplicate/missing IDs, finite score
validation, score-descending stable ordering, invalid input as hard error, and
backend timeout/invalid response as RRF fallback with a warning.

- [ ] **Step 2: Run reranker tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestRerank'`.
Expected failure: reranker symbols are missing.

- [ ] **Step 3: Implement reranker domain and Ollama adapter**

Send temperature 0 to `/api/generate`, request JSON output, and parse only:

```json
{"scores":[{"candidate_id":"scope/chunk","relevance":0.75}]}
```

The prompt marks candidate text as untrusted data and forbids following its
instructions. The HTTP response must map every original ID exactly once.

- [ ] **Step 4: Write and implement diversity through RED/GREEN**

Tests prove maximum 3 per document and 2 per heading, skipped reasons, stable
order, no cross-scope mixing, and no mutation of input. Implement a single pass
over reranked candidates.

- [ ] **Step 5: Run real reranker and forced fallback**

Evaluate validation Hybrid with local `qwen:7b`, then run the command against a
deliberately unavailable local endpoint and prove the same RRF candidate order
returns with a warning. Record model, call count, candidates, quality and
reranker p50/p95. Do not enable Reranker by default unless validation NDCG@10
improves.

- [ ] **Step 6: Document, verify, and commit L37**

Run focused tests, package tests, `gofmt`, and `git diff --check`.
Commit: `feat: add reranker and retrieval diversity`.

## Task 5: L38 Calibrated Retrieval Decision Policy

**Files:**

- Create: `internal/retrievalquality/calibration.go`
- Create: `internal/retrievalquality/calibration_test.go`
- Create: `internal/retrievalquality/policy.go`
- Create: `internal/retrievalquality/policy_test.go`
- Create: `internal/retrievalquality/regression.go`
- Create: `internal/retrievalquality/regression_test.go`
- Create: `cmd/retrieval-quality-demo/policy.go`
- Create: `docs/teaching/retrieval-decision-policy-sop.md`
- Modify: `docs/teaching/00-retrieval-quality-batch-operation-log.md`
- Modify: `docs/teaching/00-course-blueprint.md`
- Modify: `docs/teaching/00-learning-status.md`
- Modify: `docs/teaching/00-handoff-guide.md`
- Modify: `docs/teaching/00-optimization-backlog.md`
- Modify: `world/game-world-map.md`

**Interfaces:**

- Consumes: fixed train/validation reports and all earlier strategies
- Produces:
  - `FitIsotonic([]CalibrationSample) (Calibrator, error)`
  - `Calibrator.Predict(float64) float64`
  - `SelectPolicy(dataset Dataset, trainOutcomes []Outcome, grid []PolicyParams) (Policy, error)`
  - `Policy.ValidateIdentity(Dataset, EncoderIdentity) error`
  - `CheckRegression(baseline, candidate Report, maxLatencyRatio float64) RegressionResult`

- [ ] **Step 1: Write calibration RED tests**

Prove PAVA merges decreasing adjacent blocks, prediction is monotonic and
clamped, repeated scores aggregate deterministically, Brier/ECE are correct, and
NaN/Inf/empty samples fail.

- [ ] **Step 2: Run calibration tests and verify RED**

Run `go test ./internal/retrievalquality -run 'TestFitIsotonic|TestBrier|TestECE'`.
Expected failure: calibration symbols are missing.

- [ ] **Step 3: Implement PAVA and calibration metrics**

Sort by score with deterministic stable input order, aggregate equal scores, run
weighted adjacent-violators pooling, and serialize breakpoints/values.

- [ ] **Step 4: Write policy and regression RED tests**

Use a trap validation outcome that would change the winner if read; prove
selection ignores it. Prove exact/code/semantic/mixed/unknown routing, stable
grid tie breaks, checksum/encoder mismatch rejection, deterministic policy
checksum, and every regression gate from the design.

- [ ] **Step 5: Implement train-only policy selection**

Grid values are finite and checked in:

```text
dense_weight: 0.5, 1.0, 2.0
sparse_weight: 0.5, 1.0, 2.0
candidate_quota: 5, 10, 20
rerank_enabled: false, true
minimum_relevance: 0.0, 0.25, 0.5
```

Rank candidates by train NDCG@10, Recall@10, lower p95, then canonical parameter
JSON. Persist the selected per-kind routes, calibrators, identities, reasons and
policy checksum.

- [ ] **Step 6: Run real policy fit and validation gate**

Fit only with train outcomes, write the policy artifact under ignored local
output or print it, then evaluate validation once. Run a second time and prove
the policy checksum and rank decisions match. Force Sparse and Reranker failures
separately and record deterministic fallback.

- [ ] **Step 7: Update teaching navigation and world map**

Set L34-L38 to `已实现待学习`, implementation boundary to L38, next new practice
to L39, and leave next teaching lesson at L24. Mark the mapped optimization items
implemented without claiming production generalization.

- [ ] **Step 8: Document, verify, and commit L38**

Run focused tests, `go test ./internal/retrievalquality`, `gofmt`, and
`git diff --check`. Commit: `feat: add calibrated retrieval policy`.

## Task 6: Batch Verification and Completion Audit

**Files:**

- Modify only if evidence changes: `docs/teaching/00-retrieval-quality-batch-operation-log.md`

- [ ] **Step 1: Read verification skill and inspect current diff/history**

Confirm five lesson commits exist after the design/plan commits and unrelated
untracked files remain unstaged.

- [ ] **Step 2: Run the full automated suite**

Run with repository-local Go cache:

```bash
env GOCACHE=/Users/huangyanyu/offline-rag-go-lab/.cache/go-build \
  GOSUMDB=off go test ./...
```

No package may fail.

- [ ] **Step 3: Re-run the six real commands**

Run dataset, index, sparse, hybrid, rerank, and policy commands against the exact
v1 dataset and isolated collection. Inspect Qdrant alias and collection metadata;
inspect Ollama model metadata. Compare output identities and gates to the design.

- [ ] **Step 4: Audit every explicit requirement**

Map the design sections and active goal to files, tests, command output, commits,
SOPs, operation-log evidence, and navigation updates. Treat missing or indirect
evidence as incomplete and fix it before completion.

- [ ] **Step 5: Commit evidence correction if required**

If final reruns change only observed metrics or timestamps, commit the operation
log as `docs: record retrieval quality verification`. Do not amend lesson commits.

- [ ] **Step 6: Mark the goal complete only after the audit passes**

Do not push. Report commit hashes, real quality/latency results, fallback evidence,
full-test result, and the next implementation/teaching coordinates.
