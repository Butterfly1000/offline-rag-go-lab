# Production Document Ingestion Five Lessons Design

**Date:** 2026-07-17

**Status:** Approved: local MySQL + local Qdrant + local Ollama

## 1. Goal

Complete lessons 29-33 so the project can turn Markdown and Go source files into
versioned, structure-aware chunks, ingest them idempotently, rebuild a Qdrant
snapshot behind an alias, and measure retrieval quality with a reviewed golden
set.

Each lesson must include:

- focused Go code with comments for non-obvious behavior
- a RED test observed failing before the production implementation
- passing unit tests and a runnable command or curl path
- a teaching SOP under `docs/teaching`
- current implementation and production boundary notes
- review and an independent commit

Implementation completion must not be recorded as user learning completion.

## 2. Approved Local Boundary

Only these local services may be used without another approval:

- MySQL at `127.0.0.1:3306`, database `offline_rag`
- Qdrant at `http://127.0.0.1:6333`
- Ollama at `http://127.0.0.1:11434`, embedding model `bge-m3`
- the repository tokenizer at `assets/tokenizers/qwen2/tokenizer.json`

Configuration continues to come from ignored `config/recent-chat.env`; checked-in
examples contain no credentials. No cloud database, remote model API, push, or
write outside this repository and the three approved local services is allowed.

The existing `offline_rag_document_chunks_v1` collection remains untouched. This
chapter uses isolated resources:

- physical collections `offline_rag_document_ingestion_lab_v1` and `_v2`
- stable alias `offline_rag_document_ingestion_lab_active`
- MySQL tables `document_sources`, `document_versions`, and
  `document_chunk_manifests`

The chapter never deletes an existing physical collection. Alias rollback points
the stable name back to the previous verified collection.

## 3. Selected Architecture

MySQL is the source of truth for document identity, versions, ingestion state and
the expected chunk manifest. Qdrant is a derived search index and stores vectors
plus enough payload to validate a search result. Original source files remain the
content source; this chapter does not copy complete files into MySQL.

The data flow is:

```text
local source file
  -> canonical document/version identity
  -> Markdown or Go structure parser
  -> exact tokenizer-bounded chunks
  -> local bge-m3 embeddings
  -> physical Qdrant collection
  -> MySQL manifest/status update
  -> collection validation
  -> atomic alias switch
  -> golden-query evaluation
```

An interface separates the domain service from MySQL, Ollama and Qdrant clients.
Unit tests use fakes or `httptest`; real local services are exercised only by the
teaching commands.

## 4. Document and Version Identity

A logical document is owned by the pair `(knowledge_scope, document_id)`.
`document_id` is supplied explicitly by the operator instead of being derived
from an absolute machine path. `source_ref` is a portable repository-relative
reference shown in citations.

The normalized source bytes produce a lowercase SHA256 `content_hash`. A repeated
ingestion with the same scope, document ID, parser version, chunk policy and
content hash is a no-op. A changed input creates an immutable document version.

The MySQL records are:

```text
document_sources
  id, knowledge_scope, document_id, source_ref,
  active_version_id, created_at, updated_at

document_versions
  id, document_source_id, content_hash, parser_version, chunk_policy_hash,
  status, target_collection, chunk_count, error_message,
  created_at, activated_at, updated_at

document_chunk_manifests
  document_version_id, chunk_id, structure_kind, heading_path,
  ordinal, content_hash, token_count, qdrant_point_id, created_at
```

Allowed version transitions are:

```text
pending -> building -> ready -> active
                    -> failed
failed  -> building
active versions remain immutable
```

MySQL unique keys prevent duplicate logical sources, duplicate build definitions
and duplicate chunk identities within one version. State updates use explicit
expected-state conditions so two workers cannot silently activate conflicting
versions. The version and chunk tables use forward foreign keys. The nullable
`active_version_id` pointer is checked inside the activation transaction rather
than using a circular foreign key that a plain idempotent teaching schema cannot
reapply safely.

## 5. Stable Chunk Identity

`chunk_id` must remain stable when an unchanged section survives a new document
version. It therefore does not contain `document_version_id` or a global line
number.

The canonical identity input is:

```text
knowledge_scope NUL document_id NUL structure_kind NUL heading_path NUL
normalized_chunk_content_hash NUL duplicate_ordinal
```

SHA256 of this input is encoded as lowercase hexadecimal. The duplicate ordinal
only distinguishes identical chunks under the same structural path. The Qdrant
point ID continues to use the existing deterministic UUID conversion from
`knowledge_scope + chunk_id`.

Consequences are intentional:

- unchanged content under the same heading/declaration keeps its chunk ID
- changed content receives a new chunk ID
- moving content to another heading/declaration receives a new chunk ID
- inserting unrelated content earlier in the file does not rename later chunks
- identical repeated blocks remain distinct and deterministic

## 6. Structure-Aware Chunking

Create a focused `internal/documentingest` package. It accepts a document, a
format, a token counter and a chunk policy. The first production-supported
formats are Markdown and Go source.

Markdown behavior:

1. ATX headings update a heading stack; headings are metadata, not standalone
   empty chunks.
2. Blank-line-delimited paragraphs remain atomic when they fit.
3. Fenced code blocks remain atomic and retain their language marker when they
   fit.
4. Adjacent compatible blocks under one heading are packed up to `max_tokens`.
5. Oversized paragraphs are split by sentence, then by tokenizer-safe text spans.
6. Oversized code blocks are split by complete lines with configurable overlap.

Go behavior:

1. `go/parser` and token offsets locate package comments and top-level
   declarations.
2. A declaration and its doc comment are kept together when they fit.
3. Oversized declarations are split by complete source lines with overlap.
4. Syntax errors fail ingestion; silently treating malformed Go as plain text is
   not allowed.

Every emitted chunk is counted with the real tokenizer. Empty chunks and chunks
over `max_tokens` are rejected. Overlap is applied only when a single structural
unit must be split, not between every adjacent section.

This chapter deliberately does not support PDF, HTML, Office files, semantic LLM
chunking or tree-sitter for multiple languages. Those belong in the optimization
backlog after the Markdown/Go framework is measured.

## 7. Idempotent Ingestion

The ingestion service receives one source document and a target physical
collection. Its order is:

1. validate scope, document ID, portable source reference and target collection
2. hash the complete source and chunk policy
3. create or load the immutable version record
4. return `noop` if the same build is already ready or active
5. claim `pending` or `failed` as `building`
6. parse and chunk using the real tokenizer
7. embed chunks in bounded batches with local Ollama
8. upsert deterministic points into the target physical collection
9. save the complete MySQL chunk manifest
10. mark the version `ready` only after point and manifest counts agree
11. record `failed` with a bounded error message on recoverable failure

A retry reuses the same version and deterministic point IDs. Repeating a partial
batch overwrites the same points rather than creating duplicates. Cancellation is
propagated to MySQL, Ollama and Qdrant requests.

Incremental stale-point deletion is implemented and tested as an explicit
operation, but the recommended production publication path is the alias rebuild
in lesson 32. The real teaching run does not delete physical collections.

## 8. Snapshot Rebuild and Alias Recovery

The rebuild command writes a complete expected corpus into an empty physical
collection, then verifies:

- vector size is 1024 and distance is Cosine
- required payload indexes exist
- every ready manifest point exists in Qdrant
- actual point count equals expected manifest count
- a sample of payload identities and content hashes revalidates

Only a verified collection can be activated. Qdrant's alias update API removes
the alias from the old physical collection and adds it to the new one in one
request. MySQL active-version updates happen after the alias request succeeds.
If the MySQL activation step fails, the command reports a reconciliation error
and provides a deterministic rollback command; it does not pretend the operation
was atomic across two independent databases.

Rollback performs another atomic Qdrant alias update back to the previous
collection, then reconciles MySQL active versions. Collection deletion and
automatic garbage collection are explicitly deferred.

## 9. Retrieval Evaluation

Store a checked-in golden dataset with at least ten cases. Each case contains:

```text
case_id, query, knowledge_scope, expected_chunk_ids,
forbidden_chunk_ids, notes
```

The evaluator embeds each query once, searches the active alias with a mandatory
scope filter and calculates:

- Recall@3: fraction of expected chunk IDs found in the first three results
- MRR@3: reciprocal rank of the first expected result, or zero
- scope isolation: no returned result may belong to another scope
- forbidden-hit count: explicitly prohibited chunk IDs must remain absent

The acceptance gate is:

- at least 10 reviewed cases execute
- every case has Recall@3 equal to 1.0
- aggregate scope isolation is 100%
- forbidden-hit count is zero
- the report prints aggregate MRR@3 for comparison, without inventing an
  arbitrary quality claim from the small teaching corpus

Fixture documents contain positive, paraphrased and competing-scope examples.
The evaluator exits non-zero when any gate fails and emits deterministic JSON so
later chunking, quota or reranking experiments can compare against the baseline.

## 10. Lesson Structure

### Lesson 29: Document Identity and Version State

Implement and test canonical hashes, stable chunk IDs, version transitions and
MySQL schema/store boundaries. A pure local demo prints unchanged/changed/moved
chunk identity comparisons before any database write is required.

### Lesson 30: Markdown and Go Structure-Aware Chunking

Implement the parsers and exact token policy. Practice runs one Markdown fixture
and one Go fixture, printing heading/declaration paths, token counts and chunk
IDs. Tests prove fenced-code preservation, declaration boundaries, oversized
splitting and hard token limits.

### Lesson 31: Real Idempotent Ingestion

Implement the MySQL store, batched local Ollama embeddings and Qdrant batch
upsert/delete APIs. The SOP applies the three local tables, creates the isolated
`_v1` collection, ingests the fixtures twice and proves the second run is a no-op
with unchanged point and manifest counts.

### Lesson 32: Verified Rebuild, Alias Switch and Rollback

Implement `_v2` full rebuild, verification, atomic Qdrant alias switch and
explicit rollback/reconciliation. The SOP proves queries through
`offline_rag_document_ingestion_lab_active` move from `_v1` to `_v2` and can move
back without deleting either collection.

### Lesson 33: Golden Retrieval Evaluation

Implement at least ten golden cases, Recall@3/MRR@3/isolation metrics and a JSON
report. The SOP runs the evaluator against the local active alias and records the
observed baseline. Chunk-policy or fixture corrections are allowed only when the
case remains a legitimate retrieval test, not to hide a wrong-scope result.

## 11. Error and Security Boundaries

Hard failures include invalid identity, path traversal in `source_ref`, malformed
Go, oversized chunks after fallback splitting, vector dimension mismatch,
manifest/point mismatch, illegal state transitions, aliasing an unverified
collection and any scope mismatch returned by Qdrant.

Operational failures include local MySQL, Qdrant or Ollama unavailability. They
mark the claimed version failed when possible and preserve the previously active
alias. Error bodies and saved failure messages are bounded. Credentials, absolute
home paths and complete source contents are not logged.

Search always filters `knowledge_scope`, then revalidates payload scope, point ID,
content hash and embedding model after Qdrant returns results.

## 12. Review and Verification Gates

Every lesson follows:

```text
RED -> verify expected failure -> GREEN -> focused tests -> runnable practice
-> SOP -> gofmt/diff check -> review -> independent commit
```

Per-lesson review checks:

- deterministic identity and ordering
- version transition and retry correctness
- mandatory knowledge-scope isolation
- bounded HTTP clients, batches, errors and context cancellation
- no writes to existing memory or document collections
- no real credentials or full machine paths in tracked files
- SOP commands and observed output match the implementation

Final gates are:

```bash
go test ./...
go test -race ./internal/documentingest ./internal/contextretrieval
go vet ./...
go build ./...
gofmt -d <changed Go files>
git diff --check
```

The final worktree must be clean. Five lesson commits are required in addition to
design and plan commits. No push is performed.

## 13. Deferred Optimization

Record but do not implement in this chapter:

- PDF, HTML, Office and additional programming-language parsers
- distributed worker leasing and heartbeat recovery
- automatic old-collection deletion and retention policy
- cross-encoder reranking, hybrid BM25/vector retrieval and score calibration
- dynamic chunk policies per document class
- large-corpus latency, throughput and embedding-cost benchmarks
