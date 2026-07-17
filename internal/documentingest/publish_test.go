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
	validateErr error
	activateErr error
	validated   []SnapshotVersion
	activated   []SnapshotVersion
	active      map[int64]int64
}

func (f *fakePublicationStore) ValidateSnapshotVersions(_ context.Context, _ string, _ string, versions []SnapshotVersion) error {
	f.validated = append([]SnapshotVersion(nil), versions...)
	return f.validateErr
}

func (f *fakePublicationStore) ActivateSnapshot(_ context.Context, _ string, _ string, versions []SnapshotVersion) error {
	f.activated = append([]SnapshotVersion(nil), versions...)
	if f.active != nil {
		for sourceID := range f.active {
			delete(f.active, sourceID)
		}
		for _, version := range versions {
			f.active[version.DocumentSourceID] = version.VersionID
		}
	}
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
	if err == nil || !result.AliasSwitched || !result.ReconciliationRequired || len(store.activated) != 1 {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

func TestPublisherRollbackRequiresExpectedCurrentAlias(t *testing.T) {
	index := validSnapshotIndex()
	publisher := Publisher{Index: index, Store: &fakePublicationStore{}, Now: time.Now}
	_, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: "unexpected", Snapshot: rollbackSnapshot()})
	if err == nil || len(index.switches) != 0 {
		t.Fatalf("error=%v switches=%v", err, index.switches)
	}
}

func TestPublisherRollbackUpdatesAliasAndMySQLActiveVersions(t *testing.T) {
	index := validSnapshotIndex()
	index.alias = "offline_rag_document_ingestion_lab_v2"
	store := &fakePublicationStore{}
	publisher := Publisher{Index: index, Store: store, Now: time.Now}

	result, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: rollbackSnapshot()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.AliasSwitched || !result.MySQLActivated || result.ReconciliationRequired {
		t.Fatalf("result=%#v", result)
	}
	if len(store.activated) != 1 || store.activated[0].VersionID != 1 || index.alias != "offline_rag_document_ingestion_lab_v1" {
		t.Fatalf("activated=%#v alias=%q", store.activated, index.alias)
	}
}

func TestPublisherRollbackClearsSourcesMissingFromTargetSnapshot(t *testing.T) {
	index := validSnapshotIndex()
	index.alias = "offline_rag_document_ingestion_lab_v2"
	store := &fakePublicationStore{active: map[int64]int64{1: 2, 2: 3}}
	publisher := Publisher{Index: index, Store: store, Now: time.Now}

	result, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: rollbackSnapshot()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.MySQLActivated || len(store.active) != 1 || store.active[1] != 1 {
		t.Fatalf("result=%#v active=%#v", result, store.active)
	}
}

func TestPublisherRollbackRejectsSnapshotOwnershipBeforeAliasSwitch(t *testing.T) {
	index := validSnapshotIndex()
	index.alias = "offline_rag_document_ingestion_lab_v2"
	store := &fakePublicationStore{validateErr: errors.New("version belongs to another scope")}
	publisher := Publisher{Index: index, Store: store, Now: time.Now}

	_, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: rollbackSnapshot()})
	if err == nil || len(index.switches) != 0 || len(store.activated) != 0 {
		t.Fatalf("error=%v switches=%v activated=%#v", err, index.switches, store.activated)
	}
}

func TestPublisherRollbackRejectsDuplicateSourceBeforeAliasSwitch(t *testing.T) {
	index := validSnapshotIndex()
	index.alias = "offline_rag_document_ingestion_lab_v2"
	store := &fakePublicationStore{}
	publisher := Publisher{Index: index, Store: store, Now: time.Now}
	snapshot := rollbackSnapshot()
	snapshot.Versions = []SnapshotVersion{{DocumentSourceID: 1, VersionID: 1}, {DocumentSourceID: 1, VersionID: 2}}
	snapshot.VersionID = 0
	snapshot.DocumentSourceID = 0

	_, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: snapshot})
	if err == nil || len(index.switches) != 0 || len(store.validated) != 0 {
		t.Fatalf("error=%v switches=%v validated=%#v", err, index.switches, store.validated)
	}
}

func TestPublisherRollbackReportsMySQLReconciliationAfterAliasSwitch(t *testing.T) {
	index := validSnapshotIndex()
	index.alias = "offline_rag_document_ingestion_lab_v2"
	store := &fakePublicationStore{activateErr: errors.New("mysql unavailable")}
	publisher := Publisher{Index: index, Store: store, Now: time.Now}

	result, err := publisher.Rollback(context.Background(), RollbackRequest{Alias: "offline_rag_document_ingestion_lab_active", From: index.alias, Snapshot: rollbackSnapshot()})
	if err == nil || !result.AliasSwitched || !result.ReconciliationRequired || result.MySQLActivated {
		t.Fatalf("result=%#v error=%v", result, err)
	}
}

func rollbackSnapshot() SnapshotManifest {
	snapshot := validSnapshot()
	snapshot.VersionID = 1
	snapshot.Collection = "offline_rag_document_ingestion_lab_v1"
	return snapshot
}
