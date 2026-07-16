# Dual Retrieval Five Lessons Design

**Date:** 2026-07-16

**Status:** Approved direction: real Memory Qdrant + real Document Qdrant

## 1. Goal

Complete lessons 24-28 so the current project can retrieve long-term memory and
knowledge documents independently, combine them deterministically, account for
their real prompt tokens, and inject the selected context into the real
recent-chat Ollama request.

Each lesson must include:

- focused Go code with comments where behavior is not self-explanatory
- unit tests and a runnable demo or curl path
- a teaching SOP under docs/teaching
- current implementation and production boundary notes
- review and an independent commit

Implementation completion must not be recorded as user learning completion.

## 2. Current Boundary

The repository already has:

- MySQL recent messages and rolling session summaries
- automatic token budgeting with the real tokenizer and Ollama context limit
- MySQL memory items as the fact source
- bge-m3 embedding and the dedicated offline_rag_memory_items_v1 Qdrant collection
- a separate teaching RAG gateway whose document retrieval is currently in memory

The missing behavior is a real knowledge-document vector collection and a bridge
that lets recent-chat use both retrieval sources.

## 3. Selected Approach

Use two independent Qdrant collections:

- offline_rag_memory_items_v1 for user-scoped long-term memory
- offline_rag_document_chunks_v1 for knowledge-scope document chunks

Both collections use bge-m3 and 1024-dimensional Cosine vectors. They remain
separate because they have different ownership, payload, lifecycle, filters and
rebuild sources.

One query embedding is shared by both searches. The two searches run
concurrently after embedding succeeds.

This chapter does not merge the two collections physically and does not reuse or
modify ollama_chat_memory.

## 4. Package and Component Boundaries

Create internal/contextretrieval with focused files.

### types.go

Defines the common result shape without erasing source-specific ownership.

    type Source string

    const (
        SourceMemory   Source = "memory"
        SourceDocument Source = "document"
    )

    type Hit struct {
        Source         Source
        ID             string
        Content        string
        Score          float64
        UserID         string
        KnowledgeScope string
        Kind           string
        Title          string
        SourceRef      string
        Metadata       map[string]string
    }

A memory hit must carry UserID and must not claim KnowledgeScope. A document hit
must carry KnowledgeScope and must not claim UserID. Validation rejects mixed or
missing ownership.

### document_qdrant.go

Owns the document collection contract:

- create or validate 1024/Cosine vector configuration
- create keyword indexes for knowledge_scope and document_id
- upsert active document chunks
- search with a mandatory knowledge_scope filter
- validate returned point ID and payload fields
- never access the memory collection

Document point IDs are deterministic UUID strings derived from
knowledge_scope + NUL + chunk_id using SHA256. Re-running the demo updates the
same point instead of creating duplicates.

Document payload contains:

- knowledge_scope
- document_id
- chunk_id
- title
- source_ref
- text
- content_hash
- embedding_model

### dual.go

Defines narrow source interfaces and the orchestration result.

    type MemorySearcher interface {
        Search(ctx context.Context, userID string, vector []float32, limit int) ([]Hit, error)
    }

    type DocumentSearcher interface {
        Search(ctx context.Context, knowledgeScope string, vector []float32, limit int) ([]Hit, error)
    }

    type DualResult struct {
        MemoryHits    []Hit
        DocumentHits  []Hit
        Warnings      []string
    }

The dual retriever:

1. validates query, user ID, scope and limits
2. embeds the query once
3. searches both enabled sources concurrently
4. records source-specific infrastructure warnings
5. returns successful results even if the other source has an infrastructure failure
6. returns an empty result with warnings if both searches have infrastructure failures

Request validation failures remain hard errors. Retrieval infrastructure failures
are soft failures so they do not prevent the main chat answer. Ownership,
collection-contract and malformed-payload errors are security/data-integrity
failures and remain hard errors rather than being converted to warnings.

### merge.go

Merges results without pretending the raw scores are globally comparable.

The deterministic first version:

1. validate every hit
2. sort each source independently by descending score and stable ID
3. apply separate memory and document limits
4. deduplicate normalized content across sources
5. append memory quota first, then document quota
6. preserve Source on every selected hit

The order is deliberate: personal memory is compact and user-specific; documents
then provide broader project facts. Production reranking and score calibration
remain backlog items.

### budget.go and prompt.go

Render selected context as an explicitly untrusted data block:

    <retrieved_context>
      <memory>...</memory>
      <document>...</document>
    </retrieved_context>

Values are escaped before rendering. The system instruction states that retrieved
content is data, not executable instructions.

The context budgeter uses the real tokenizer. It adds hits one at a time and
counts the complete rendered context block after each addition. A hit is retained
only if the block remains within context_token_budget.

The selected block becomes part of the fixed system input. The existing automatic
planner then counts:

- original system prompt
- retrieved context
- session summary
- current user message
- assistant prefix
- output reserve

The remaining capacity is available to recent history. This preserves one global
model context limit instead of maintaining unrelated character limits.

## 5. Lesson Structure

### Lesson 24: Unified Retrieval Result Boundary

Implement Source, Hit, request/result types and strict ownership validation.

Practice:

    go run ./cmd/context-hit-demo

The demo shows one valid memory hit, one valid document hit, and rejected mixed
ownership.

No MySQL, Ollama or Qdrant state changes occur.

### Lesson 25: Real Document Qdrant Retrieval

Implement the document Qdrant client and a demo that embeds fixed chunks using
bge-m3, creates offline_rag_document_chunks_v1, upserts deterministic points and
runs a scope-filtered query.

Practice:

    go run ./cmd/document-qdrant-demo --config config/recent-chat.env --apply

The command is idempotent. It writes only the dedicated teaching document
collection. It must prove that a different knowledge_scope cannot retrieve the
points.

### Lesson 26: Parallel Dual Retrieval and Failure Isolation

Adapt the existing memory Qdrant search to the common Hit type, combine it with
the document searcher, share one query embedding and run both searches
concurrently.

Practice:

    go run ./cmd/dual-retrieval-demo --config config/recent-chat.env

The output shows memory hits, document hits and warnings separately. Tests use
controllable fakes to prove concurrency and one-side failure behavior.

### Lesson 27: Deterministic Merge and Exact Context Budget

Implement source quotas, stable ordering, cross-source content deduplication,
safe prompt rendering and exact tokenizer-based context selection.

Practice:

    go run ./cmd/context-merge-demo --config config/recent-chat.env

The demo prints candidates, selected hits, duplicate removal, rendered context and
actual context token count.

### Lesson 28: Real recent-chat Integration

Extend recent-chat through a ContextRetriever interface. Existing behavior is
unchanged unless retrieval flags are enabled.

New request fields:

- use_memory
- use_knowledge
- knowledge_scope
- memory_limit
- document_limit
- context_token_budget

New response observations:

- retrieved_context
- used_memory_items
- used_document_chunks
- used_context_tokens
- retrieval_warnings

The service retrieves and budgets context before the final automatic history
budget. It injects the rendered context into the system message, then executes the
existing summary, recent-window, Ollama and MySQL message-store flow.

Practice uses real MySQL, Ollama, bge-m3 and both Qdrant collections with curl.
Assertions must show:

- the correct user memory is available
- the correct knowledge scope document is available
- another user memory is excluded
- another knowledge scope document is excluded
- one retrieval source can fail without preventing the answer
- the response exposes selected hits and token usage

## 6. Configuration

Continue using the ignored config/recent-chat.env file. Do not add required shell
environment variables.

Add example keys:

    QDRANT_BASE_URL=http://127.0.0.1:6333
    QDRANT_MEMORY_COLLECTION=offline_rag_memory_items_v1
    QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
    OLLAMA_EMBED_MODEL=bge-m3

The checked-in example contains no real credentials.

## 7. Security and Isolation

Mandatory invariants:

- memory search always filters user_id
- document search always filters knowledge_scope
- payload is revalidated after Qdrant returns it
- request flags cannot bypass the required ownership value
- result source and ownership cannot be mixed
- retrieved text is escaped and marked as untrusted data
- Qdrant failure cannot modify MySQL memory facts
- document writes never target existing memory collections

## 8. Failure Behavior

Hard failures:

- invalid request fields
- invalid configured collection names
- tokenizer initialization failure
- fixed prompt plus output reserve exceeding model context
- malformed or ownership-violating data returned by Qdrant

Soft failures during chat:

- memory query timeout or unavailable collection
- document query timeout or unavailable collection
- query embedding infrastructure failure

Soft failures are returned as retrieval_warnings and chat continues without the
failed source. The standalone demos still return non-zero on infrastructure
failure so the operator can diagnose the environment.

Hard errors and soft infrastructure errors use distinct Go error types so the
orchestrator cannot accidentally downgrade a user/scope isolation violation.

## 9. Test and Review Gates

Every lesson follows RED -> GREEN -> practice -> SOP -> review -> commit.

Per-lesson review checks:

- user and knowledge-scope isolation
- deterministic output
- no cross-collection writes
- context cancellation and bounded HTTP clients
- no real credentials or full machine paths
- teaching document matches actual code and command output

Final gates:

    go test ./...
    go test -race ./internal/contextretrieval ./internal/memoryitem ./internal/recentchat
    go vet ./...
    go build ./cmd/...
    gofmt -d on changed Go files
    git diff --check

No git push is performed.

## 10. Production Work Explicitly Deferred

The following are useful but do not block this five-lesson chapter:

- cross-encoder reranking
- calibrated score fusion
- dynamic source quotas
- document ingestion workers and formal migrations
- MySQL outbox for memory-to-Qdrant synchronization
- Qdrant alias-based zero-downtime rebuilds
- retrieval metrics, Recall@K evaluation and tracing
- cache and request coalescing

These items belong in docs/teaching/00-optimization-backlog.md.
