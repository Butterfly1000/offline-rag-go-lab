package memoryitem

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type fakeMemorySQL struct {
	row    RowScanner
	rows   RowsScanner
	result sql.Result
	err    error
	query  string
	args   []any
}

func (db *fakeMemorySQL) QueryRowContext(_ context.Context, query string, args ...any) RowScanner {
	db.query, db.args = query, args
	return db.row
}

func (db *fakeMemorySQL) QueryContext(_ context.Context, query string, args ...any) (RowsScanner, error) {
	db.query, db.args = query, args
	return db.rows, db.err
}

func (db *fakeMemorySQL) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.query, db.args = query, args
	return db.result, db.err
}

type fakeMemoryRow struct {
	item Item
	err  error
}

func (r fakeMemoryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	assignItemScan(dest, r.item)
	return nil
}

type fakeMemoryRows struct {
	items []Item
	index int
	err   error
}

func (r *fakeMemoryRows) Next() bool { return r.index < len(r.items) }
func (r *fakeMemoryRows) Scan(dest ...any) error {
	assignItemScan(dest, r.items[r.index])
	r.index++
	return nil
}
func (r *fakeMemoryRows) Err() error   { return r.err }
func (r *fakeMemoryRows) Close() error { return nil }

func assignItemScan(dest []any, item Item) {
	*(dest[0].(*int64)) = item.ID
	*(dest[1].(*string)) = item.UserID
	*(dest[2].(*Kind)) = item.Kind
	*(dest[3].(*string)) = item.Key
	*(dest[4].(*string)) = item.Value
	*(dest[5].(*Status)) = item.Status
	*(dest[6].(*int64)) = item.Version
	*(dest[7].(*time.Time)) = item.CreatedAt
	*(dest[8].(*time.Time)) = item.UpdatedAt
}

type fakeMemorySQLResult struct {
	insertID int64
	affected int64
	err      error
}

func (r fakeMemorySQLResult) LastInsertId() (int64, error) { return r.insertID, r.err }
func (r fakeMemorySQLResult) RowsAffected() (int64, error) { return r.affected, r.err }

func TestMySQLMemoryQueriesFindAndLockAreUserScoped(t *testing.T) {
	want := persistedMemoryItem(StatusActive, "Go", 2)
	db := &fakeMemorySQL{row: fakeMemoryRow{item: want}}
	queries := newMySQLMemoryQueries(db)

	got, err := queries.FindForUpdate(context.Background(), "u-001", KindProjectFact, "implementation_language")
	if err != nil || got != want {
		t.Fatalf("FindForUpdate() item=%#v error=%v", got, err)
	}
	for _, fragment := range []string{"FROM memory_items", "user_id = ?", "kind = ?", "memory_key = ?", "FOR UPDATE"} {
		if !strings.Contains(db.query, fragment) {
			t.Fatalf("query lacks %q: %s", fragment, db.query)
		}
	}
	if len(db.args) != 3 || db.args[0] != "u-001" {
		t.Fatalf("args = %#v", db.args)
	}
}

func TestMySQLMemoryQueriesListActiveFiltersUserAndStatus(t *testing.T) {
	rows := &fakeMemoryRows{items: []Item{persistedMemoryItem(StatusActive, "Go", 2)}}
	db := &fakeMemorySQL{rows: rows}
	items, err := newMySQLMemoryQueries(db).ListActive(context.Background(), "u-001")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListActive() items=%#v error=%v", items, err)
	}
	if !strings.Contains(db.query, "user_id = ?") || !strings.Contains(db.query, "status = 'active'") || !strings.Contains(db.query, "ORDER BY id") {
		t.Fatalf("query = %s", db.query)
	}
}

func TestMySQLMemoryQueriesUpdateUsesIdentityAndVersionGuards(t *testing.T) {
	db := &fakeMemorySQL{result: fakeMemorySQLResult{affected: 1}}
	item := persistedMemoryItem(StatusActive, "Go", 3)
	affected, err := newMySQLMemoryQueries(db).UpdateItem(context.Background(), item, 2)
	if err != nil || affected != 1 {
		t.Fatalf("UpdateItem() affected=%d error=%v", affected, err)
	}
	for _, fragment := range []string{"UPDATE memory_items", "id = ?", "user_id = ?", "kind = ?", "memory_key = ?", "version = ?"} {
		if !strings.Contains(db.query, fragment) {
			t.Fatalf("query lacks %q: %s", fragment, db.query)
		}
	}
	if db.args[len(db.args)-1] != int64(2) {
		t.Fatalf("args = %#v, want expected version last", db.args)
	}
}

func TestMySQLMemoryQueriesInsertItemAndEvidence(t *testing.T) {
	itemDB := &fakeMemorySQL{result: fakeMemorySQLResult{insertID: 7, affected: 1}}
	itemID, err := newMySQLMemoryQueries(itemDB).InsertItem(context.Background(), persistedMemoryItem(StatusActive, "Go", 1))
	if err != nil || itemID != 7 || !strings.Contains(itemDB.query, "INSERT INTO memory_items") {
		t.Fatalf("InsertItem() id=%d error=%v query=%s", itemID, err, itemDB.query)
	}

	evidenceDB := &fakeMemorySQL{result: fakeMemorySQLResult{affected: 1}}
	affected, err := newMySQLMemoryQueries(evidenceDB).InsertEvidence(context.Background(), Evidence{
		ItemID: 7, UserID: "u-001", SessionID: "s-001", MessageID: 101,
		Role: "user", Operation: OperationUpsert, Text: "使用 Go",
	})
	if err != nil || affected != 1 || !strings.Contains(evidenceDB.query, "INSERT INTO memory_item_evidence") {
		t.Fatalf("InsertEvidence() affected=%d error=%v query=%s", affected, err, evidenceDB.query)
	}
}

func TestMySQLMemoryQueriesRejectsMissingInsertResultAndDeduplicatesEvidence(t *testing.T) {
	if _, err := newMySQLMemoryQueries(&fakeMemorySQL{}).InsertItem(context.Background(), Item{}); err == nil {
		t.Fatal("InsertItem() error = nil, want missing SQL result error")
	}

	duplicate := &fakeMemorySQL{err: &mysql.MySQLError{Number: 1062, Message: "duplicate evidence"}}
	affected, err := newMySQLMemoryQueries(duplicate).InsertEvidence(context.Background(), Evidence{})
	if err != nil || affected != 0 {
		t.Fatalf("InsertEvidence() affected=%d error=%v, want idempotent duplicate", affected, err)
	}
}
