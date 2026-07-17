package documentingest

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSnapshotIndex struct {
	info     CollectionInfo
	count    int
	points   []IndexedPoint
	alias    string
	switches [][2]string
}

func (f *fakeSnapshotIndex) CollectionInfo(context.Context, string) (CollectionInfo, error) {
	return f.info, nil
}
func (f *fakeSnapshotIndex) Count(context.Context, string, string) (int, error) { return f.count, nil }
func (f *fakeSnapshotIndex) Fetch(context.Context, string, []string) ([]IndexedPoint, error) {
	return append([]IndexedPoint(nil), f.points...), nil
}
func (f *fakeSnapshotIndex) ResolveAlias(context.Context, string) (string, error) {
	return f.alias, nil
}
func (f *fakeSnapshotIndex) SwitchAlias(_ context.Context, _ string, from, to string) error {
	f.switches = append(f.switches, [2]string{from, to})
	f.alias = to
	return nil
}

type fakePublicationStore struct {
	activateErr error
	activated   bool
}

func (f *fakePublicationStore) ActivateVersion(context.Context, int64, int64) error {
	f.activated = true
	return f.activateErr
}

func validSnapshot() SnapshotManifest {
	chunks := []ChunkManifest{
		{ChunkID: "a", ContentHash: "ha", QdrantPointID: "p1", Ordinal: 0},
		{ChunkID: "b", ContentHash: "hb", QdrantPointID: "p2", Ordinal: 1},
	}
	return SnapshotManifest{VersionID: 2, DocumentSourceID: 1, KnowledgeScope: "course", Collection: "offline_rag_document_ingestion_lab_v2", Chunks: chunks, ManifestDigest: ManifestDigest(chunks)}
}

func validSnapshotIndex() *fakeSnapshotIndex {
	return &fakeSnapshotIndex{
		info:   CollectionInfo{VectorSize: 1024, Distance: "Cosine", PayloadIndexes: map[string]bool{"knowledge_scope": true, "document_id": true, "chunk_id": true}},
		count:  2,
		points: []IndexedPoint{{ID: "p1", KnowledgeScope: "course", ChunkID: "a", ContentHash: "ha"}, {ID: "p2", KnowledgeScope: "course", ChunkID: "b", ContentHash: "hb"}},
		alias:  "offline_rag_document_ingestion_lab_v1",
	}
}

func TestPublisherVerifySnapshotChecksManifestAndIndex(t *testing.T) {
	index := validSnapshotIndex()
	publisher := Publisher{Index: index, Store: &fakePublicationStore{}, Now: time.Now}
	report, err := publisher.Verify(context.Background(), VerifyRequest{Snapshot: validSnapshot(), ExpectedVectorSize: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if report.PointCount != 2 || report.ManifestDigest != validSnapshot().ManifestDigest {
		t.Fatalf("report=%#v", report)
	}

	index.points[1].ContentHash = "wrong"
	if _, err := publisher.Verify(context.Background(), VerifyRequest{Snapshot: validSnapshot(), ExpectedVectorSize: 1024}); err == nil {
		t.Fatal("mismatched point content hash must fail verification")
	}
}

func TestPublisherActivateRejectsStaleReportAndReportsReconciliation(t *testing.T) {
	now := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	index := validSnapshotIndex()
	store := &fakePublicationStore{activateErr: errors.New("mysql unavailable")}
	publisher := Publisher{Index: index, Store: store, Now: func() time.Time { return now }, MaxVerificationAge: time.Minute}
	snapshot := validSnapshot()
	stale := VerificationReport{Collection: snapshot.Collection, KnowledgeScope: snapshot.KnowledgeScope, ManifestDigest: snapshot.ManifestDigest, VerifiedAt: now.Add(-2 * time.Minute), PointCount: 2}
	if _, err := publisher.Activate(context.Background(), ActivateRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: snapshot, Verification: stale}); err == nil {
		t.Fatal("stale verification must be rejected")
	}
	fresh := stale
	fresh.VerifiedAt = now
	result, err := publisher.Activate(context.Background(), ActivateRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: snapshot, Verification: fresh})
	if err == nil || !result.AliasSwitched || !result.ReconciliationRequired || !store.activated {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

func TestPublisherRollbackRequiresExpectedCurrentAlias(t *testing.T) {
	index := validSnapshotIndex()
	publisher := Publisher{Index: index, Store: &fakePublicationStore{}, Now: time.Now}
	err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: "unexpected", To: "offline_rag_document_ingestion_lab_v1"})
	if err == nil || len(index.switches) != 0 {
		t.Fatalf("error=%v switches=%v", err, index.switches)
	}
}
