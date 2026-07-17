package documentingest

import (
	"context"
	"errors"
	"math"
	"testing"
)

type fakeManifestStore struct {
	version    Version
	claimCalls int
	saved      []ChunkManifest
	failed     string
	markCtxErr error
}

func (s *fakeManifestStore) FindOrCreateVersion(context.Context, BuildIdentity) (Version, error) {
	return s.version, nil
}
func (s *fakeManifestStore) ClaimBuild(context.Context, int64) error { s.claimCalls++; return nil }
func (s *fakeManifestStore) SaveReadyManifest(_ context.Context, _ int64, chunks []ChunkManifest) error {
	s.saved = append([]ChunkManifest(nil), chunks...)
	return nil
}
func (s *fakeManifestStore) MarkFailed(ctx context.Context, _ int64, reason string) error {
	s.failed = reason
	s.markCtxErr = ctx.Err()
	return nil
}

type fakeBatchEmbedder struct {
	calls   int
	err     error
	vectors [][]float32
}

type cancelingBatchEmbedder struct{ cancel context.CancelFunc }

func (e cancelingBatchEmbedder) Embed(ctx context.Context, _ string, _ []string) ([][]float32, error) {
	e.cancel()
	return nil, ctx.Err()
}

func (e *fakeBatchEmbedder) Embed(_ context.Context, _ string, texts []string) ([][]float32, error) {
	e.calls++
	if e.err != nil {
		return nil, e.err
	}
	if e.vectors != nil {
		return e.vectors, nil
	}
	vectors := make([][]float32, len(texts))
	for i := range vectors {
		vectors[i] = []float32{float32(i + 1), 0.5}
	}
	return vectors, nil
}

type fakeVectorIndex struct {
	ensureCalls int
	upsertCalls int
	points      []VectorPoint
}

func (i *fakeVectorIndex) EnsureCollection(context.Context, string, int) error {
	i.ensureCalls++
	return nil
}
func (i *fakeVectorIndex) UpsertBatch(_ context.Context, _ string, points []VectorPoint) error {
	i.upsertCalls++
	i.points = append(i.points, points...)
	return nil
}
func (i *fakeVectorIndex) DeletePoints(context.Context, string, []string) error { return nil }

func testIngestRequest() IngestRequest {
	return IngestRequest{
		Document: Document{KnowledgeScope: "course", DocumentID: "intro", SourceRef: "docs/intro.md", Format: FormatMarkdown, Content: []byte("# Intro\n\nGo is used here.")},
		Policy:   ChunkPolicy{MaxTokens: 100}, ParserVersion: "markdown-v1",
		TargetCollection: "offline_rag_document_ingestion_lab_v1", EmbeddingModel: "bge-m3", BatchSize: 2,
	}
}

func TestIngestionNoopSkipsClaimEmbeddingAndQdrant(t *testing.T) {
	store := &fakeManifestStore{version: Version{ID: 7, Status: StatusReady}}
	embedder := &fakeBatchEmbedder{}
	index := &fakeVectorIndex{}
	service := IngestionService{Store: store, Index: index, Embedder: embedder, Counter: runeTokenCounter{}}
	result, err := service.Ingest(context.Background(), testIngestRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Noop || result.VersionID != 7 || store.claimCalls != 0 || embedder.calls != 0 || index.upsertCalls != 0 {
		t.Fatalf("unexpected no-op result=%#v store=%#v embed=%d index=%#v", result, store, embedder.calls, index)
	}
}

func TestIngestionBatchesAndSavesManifestAfterUpserts(t *testing.T) {
	store := &fakeManifestStore{version: Version{ID: 8, Status: StatusPending}}
	embedder := &fakeBatchEmbedder{}
	index := &fakeVectorIndex{}
	request := testIngestRequest()
	request.Policy.MaxTokens = 8
	request.BatchSize = 2
	service := IngestionService{Store: store, Index: index, Embedder: embedder, Counter: runeTokenCounter{}}
	result, err := service.Ingest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Noop || result.ChunkCount < 2 || embedder.calls < 1 || index.upsertCalls != embedder.calls {
		t.Fatalf("unexpected ingest result=%#v embed=%d upsert=%d", result, embedder.calls, index.upsertCalls)
	}
	if len(store.saved) != result.ChunkCount || len(index.points) != result.ChunkCount {
		t.Fatalf("saved=%d points=%d result=%#v", len(store.saved), len(index.points), result)
	}
}

func TestIngestionMarksBuildingVersionFailed(t *testing.T) {
	store := &fakeManifestStore{version: Version{ID: 9, Status: StatusPending}}
	embedder := &fakeBatchEmbedder{err: errors.New("embed unavailable")}
	service := IngestionService{Store: store, Index: &fakeVectorIndex{}, Embedder: embedder, Counter: runeTokenCounter{}}
	_, err := service.Ingest(context.Background(), testIngestRequest())
	if err == nil || store.failed == "" {
		t.Fatalf("error=%v failed=%q", err, store.failed)
	}
}

func TestIngestionRejectsNonFiniteEmbeddingBeforeQdrantWrite(t *testing.T) {
	store := &fakeManifestStore{version: Version{ID: 10, Status: StatusPending}}
	embedder := &fakeBatchEmbedder{vectors: [][]float32{{float32(math.NaN()), 0.5}}}
	index := &fakeVectorIndex{}
	request := testIngestRequest()
	request.BatchSize = 10
	service := IngestionService{Store: store, Index: index, Embedder: embedder, Counter: runeTokenCounter{}}
	_, err := service.Ingest(context.Background(), request)
	if err == nil || index.upsertCalls != 0 || store.failed == "" {
		t.Fatalf("error=%v upserts=%d failed=%q", err, index.upsertCalls, store.failed)
	}
}

func TestIngestionUsesCleanupContextAfterCallerCancellation(t *testing.T) {
	store := &fakeManifestStore{version: Version{ID: 11, Status: StatusPending}}
	ctx, cancel := context.WithCancel(context.Background())
	service := IngestionService{Store: store, Index: &fakeVectorIndex{}, Embedder: cancelingBatchEmbedder{cancel: cancel}, Counter: runeTokenCounter{}}
	_, err := service.Ingest(ctx, testIngestRequest())
	if err == nil || store.failed == "" || store.markCtxErr != nil {
		t.Fatalf("error=%v failed=%q cleanup context error=%v", err, store.failed, store.markCtxErr)
	}
}
