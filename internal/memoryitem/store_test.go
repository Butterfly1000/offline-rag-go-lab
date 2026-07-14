package memoryitem

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type fakeMemoryReader struct {
	item    Item
	findErr error
	items   []Item
	listErr error
	userID  string
	kind    Kind
	key     string
}

func (r *fakeMemoryReader) Find(_ context.Context, userID string, kind Kind, key string) (Item, error) {
	r.userID, r.kind, r.key = userID, kind, key
	return r.item, r.findErr
}

func (r *fakeMemoryReader) ListActive(_ context.Context, userID string) ([]Item, error) {
	r.userID = userID
	return append([]Item(nil), r.items...), r.listErr
}

type fakeMemoryTxFactory struct {
	tx       *fakeMemoryTx
	beginErr error
}

func (f *fakeMemoryTxFactory) Begin(context.Context) (MemoryUnitOfWork, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

type fakeMemoryTx struct {
	found            Item
	findErr          error
	insertID         int64
	insertErr        error
	updateAffected   int64
	updateErr        error
	evidenceAffected int64
	evidenceErr      error
	commitErr        error
	rollbackErr      error
	inserted         Item
	updated          Item
	expectedVersion  int64
	evidence         []Evidence
	committed        bool
	rollbackCalled   bool
	findUserID       string
	findKind         Kind
	findKey          string
}

func (tx *fakeMemoryTx) FindForUpdate(_ context.Context, userID string, kind Kind, key string) (Item, error) {
	tx.findUserID, tx.findKind, tx.findKey = userID, kind, key
	return tx.found, tx.findErr
}

func (tx *fakeMemoryTx) InsertItem(_ context.Context, item Item) (int64, error) {
	tx.inserted = item
	return tx.insertID, tx.insertErr
}

func (tx *fakeMemoryTx) UpdateItem(_ context.Context, item Item, expectedVersion int64) (int64, error) {
	tx.updated = item
	tx.expectedVersion = expectedVersion
	return tx.updateAffected, tx.updateErr
}

func (tx *fakeMemoryTx) InsertEvidence(_ context.Context, evidence Evidence) (int64, error) {
	tx.evidence = append(tx.evidence, evidence)
	return tx.evidenceAffected, tx.evidenceErr
}

func (tx *fakeMemoryTx) Commit() error {
	tx.committed = true
	return tx.commitErr
}

func (tx *fakeMemoryTx) Rollback() error {
	tx.rollbackCalled = true
	return tx.rollbackErr
}

func TestStoreGetAndListActiveAreUserScoped(t *testing.T) {
	reader := &fakeMemoryReader{findErr: sql.ErrNoRows, items: []Item{{ID: 7, UserID: "u-001", Status: StatusActive}}}
	store := NewStore(reader, &fakeMemoryTxFactory{})

	if _, found, err := store.Get(context.Background(), "u-001", KindProjectFact, " Implementation Language "); err != nil || found {
		t.Fatalf("Get() found=%t error=%v", found, err)
	}
	if reader.userID != "u-001" || reader.kind != KindProjectFact || reader.key != "implementation_language" {
		t.Fatalf("Find scope user=%q kind=%q key=%q", reader.userID, reader.kind, reader.key)
	}
	items, err := store.ListActive(context.Background(), "u-001")
	if err != nil || len(items) != 1 || reader.userID != "u-001" {
		t.Fatalf("ListActive() items=%#v error=%v user=%q", items, err, reader.userID)
	}
}

func TestStoreRejectsRowsOutsideRequestedUserScope(t *testing.T) {
	reader := &fakeMemoryReader{
		item:  Item{ID: 7, UserID: "u-002", Kind: KindProjectFact, Key: "implementation_language", Status: StatusActive},
		items: []Item{{ID: 8, UserID: "u-002", Status: StatusActive}},
	}
	store := NewStore(reader, &fakeMemoryTxFactory{})

	if _, _, err := store.Get(context.Background(), "u-001", KindProjectFact, "implementation_language"); err == nil {
		t.Fatal("Get() cross-user error = nil")
	}
	if _, err := store.ListActive(context.Background(), "u-001"); err == nil {
		t.Fatal("ListActive() cross-user error = nil")
	}
}

func TestStoreApplyInsertsItemAndEvidenceInOneTransaction(t *testing.T) {
	tx := &fakeMemoryTx{findErr: sql.ErrNoRows, insertID: 7, evidenceAffected: 1}
	store := newFixedTimeStore(tx)

	result, err := store.Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 101, "这个项目使用 Go。"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ActionInsert || result.Item.ID != 7 || result.Item.UserID != "u-001" || result.Item.Version != 1 {
		t.Fatalf("result = %#v", result)
	}
	if tx.inserted.ID != 0 || tx.inserted.UserID != "u-001" || tx.inserted.CreatedAt.IsZero() || tx.inserted.UpdatedAt.IsZero() {
		t.Fatalf("inserted = %#v", tx.inserted)
	}
	if result.EvidenceInserted != 1 || len(tx.evidence) != 1 || tx.evidence[0].ItemID != 7 || tx.evidence[0].MessageID != 101 {
		t.Fatalf("evidence result=%d rows=%#v", result.EvidenceInserted, tx.evidence)
	}
	if !tx.committed || tx.rollbackCalled {
		t.Fatalf("transaction committed=%t rollback=%t", tx.committed, tx.rollbackCalled)
	}
}

func TestStoreApplyNoopKeepsVersionAndCanAddEvidence(t *testing.T) {
	tx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "Go", 2), evidenceAffected: 1}
	result, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, " go ", 102, "还是使用 Go。"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ActionNoop || result.Item.Version != 2 || tx.updated.ID != 0 || result.EvidenceInserted != 1 {
		t.Fatalf("result=%#v updated=%#v", result, tx.updated)
	}
	if !tx.committed {
		t.Fatal("NOOP evidence transaction was not committed")
	}
}

func TestStoreApplyTreatsDuplicateEvidenceAsIdempotent(t *testing.T) {
	tx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "Go", 2), evidenceAffected: 0}
	result, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 102, "还是使用 Go。"))
	if err != nil {
		t.Fatal(err)
	}
	if result.EvidenceInserted != 0 || !tx.committed {
		t.Fatalf("result=%#v committed=%t", result, tx.committed)
	}
}

func TestStoreApplyUpdatesAndForgetsWithExpectedVersion(t *testing.T) {
	updateTx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "PHP", 2), updateAffected: 1, evidenceAffected: 1}
	updated, err := newFixedTimeStore(updateTx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 103, "现在项目使用 Go。"))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Decision.Action != ActionUpdate || updated.Item.Version != 3 || updateTx.expectedVersion != 2 || updateTx.updated.Value != "Go" {
		t.Fatalf("updated=%#v tx=%#v", updated, updateTx)
	}

	forgetTx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "Go", 3), updateAffected: 1, evidenceAffected: 1}
	forgotten, err := newFixedTimeStore(forgetTx).Apply(context.Background(), storeApplyRequest(OperationForget, "", 104, "请忘掉项目语言。"))
	if err != nil {
		t.Fatal(err)
	}
	if forgotten.Decision.Action != ActionForget || forgotten.Item.Status != StatusForgotten || forgotten.Item.Version != 4 || forgetTx.expectedVersion != 3 {
		t.Fatalf("forgotten=%#v tx=%#v", forgotten, forgetTx)
	}
}

func TestStoreApplyRestoresForgottenItem(t *testing.T) {
	tx := &fakeMemoryTx{found: persistedMemoryItem(StatusForgotten, "PHP", 4), updateAffected: 1, evidenceAffected: 1}
	restored, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 105, "重新记住项目使用 Go。"))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Decision.Action != ActionUpdate || restored.Item.Status != StatusActive || restored.Item.Version != 5 || tx.expectedVersion != 4 {
		t.Fatalf("restored=%#v tx=%#v", restored, tx)
	}
}

func TestStoreApplyRejectsCrossUserLockedItem(t *testing.T) {
	tx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "PHP", 2)}
	tx.found.UserID = "u-002"
	_, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 103, "现在项目使用 Go。"))
	if err == nil || !strings.Contains(err.Error(), "belongs to user") || !tx.rollbackCalled || tx.committed {
		t.Fatalf("error=%v rollback=%t committed=%t", err, tx.rollbackCalled, tx.committed)
	}
}

func TestStoreApplyMissingForgetDoesNotCreateItemOrEvidence(t *testing.T) {
	tx := &fakeMemoryTx{findErr: sql.ErrNoRows}
	result, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationForget, "", 104, "请忘掉项目语言。"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ActionNoop || result.Item.ID != 0 || tx.inserted.Kind != "" || len(tx.evidence) != 0 || !tx.committed {
		t.Fatalf("result=%#v tx=%#v", result, tx)
	}
}

func TestStoreApplyRollsBackOnEvidenceFailure(t *testing.T) {
	wantErr := errors.New("evidence failed")
	tx := &fakeMemoryTx{findErr: sql.ErrNoRows, insertID: 7, evidenceErr: wantErr}
	_, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 101, "这个项目使用 Go。"))
	if !errors.Is(err, wantErr) || !tx.rollbackCalled || tx.committed {
		t.Fatalf("error=%v rollback=%t committed=%t", err, tx.rollbackCalled, tx.committed)
	}
}

func TestStoreApplyRejectsVersionConflictAndRollsBack(t *testing.T) {
	tx := &fakeMemoryTx{found: persistedMemoryItem(StatusActive, "PHP", 2), updateAffected: 0}
	_, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 103, "现在项目使用 Go。"))
	if !errors.Is(err, ErrMemoryVersionConflict) || !tx.rollbackCalled || tx.committed {
		t.Fatalf("error=%v rollback=%t committed=%t", err, tx.rollbackCalled, tx.committed)
	}
}

func TestStoreApplyMapsDuplicateInsertToConflict(t *testing.T) {
	tx := &fakeMemoryTx{
		findErr:   sql.ErrNoRows,
		insertErr: &mysql.MySQLError{Number: 1062, Message: "duplicate memory identity"},
	}
	_, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 101, "这个项目使用 Go。"))
	if !errors.Is(err, ErrMemoryVersionConflict) || !tx.rollbackCalled || tx.committed {
		t.Fatalf("error=%v rollback=%t committed=%t", err, tx.rollbackCalled, tx.committed)
	}
}

func TestStoreApplyRollsBackWhenCommitFails(t *testing.T) {
	wantErr := errors.New("commit outcome unknown")
	tx := &fakeMemoryTx{findErr: sql.ErrNoRows, insertID: 7, evidenceAffected: 1, commitErr: wantErr}
	_, err := newFixedTimeStore(tx).Apply(context.Background(), storeApplyRequest(OperationUpsert, "Go", 101, "这个项目使用 Go。"))
	if !errors.Is(err, wantErr) || !tx.rollbackCalled || !tx.committed {
		t.Fatalf("error=%v rollback=%t commit-called=%t", err, tx.rollbackCalled, tx.committed)
	}
}

func TestStoreApplyValidatesCandidateBeforeBeginningTransaction(t *testing.T) {
	factory := &fakeMemoryTxFactory{tx: &fakeMemoryTx{}}
	store := NewStore(&fakeMemoryReader{}, factory)
	request := storeApplyRequest(OperationUpsert, "Go", 999, "这个项目使用 Go。")
	request.SourceMessages[0].ID = 101
	_, err := store.Apply(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "source message 999") {
		t.Fatalf("error=%v", err)
	}
}

func newFixedTimeStore(tx *fakeMemoryTx) *Store {
	store := NewStore(&fakeMemoryReader{}, &fakeMemoryTxFactory{tx: tx})
	store.now = func() time.Time { return time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC) }
	return store
}

func storeApplyRequest(operation Operation, value string, sourceID int64, content string) ApplyRequest {
	return ApplyRequest{
		UserID: "u-001", SessionID: "s-001",
		Candidate: Candidate{
			Operation: operation, Kind: KindProjectFact, Key: "implementation_language", Value: value,
			Confidence: 0.95, SourceMessageIDs: []int64{sourceID},
		},
		SourceMessages: []SourceMessage{{ID: sourceID, SessionID: "s-001", UserID: "u-001", Role: "user", Content: content}},
	}
}

func persistedMemoryItem(status Status, value string, version int64) Item {
	return Item{
		ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language",
		Value: value, Status: status, Version: version,
		CreatedAt: time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 7, 11, 9, 0, 0, 0, time.UTC),
	}
}
