package sessionsummary

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSummarySQL struct {
	row        RowScanner
	result     sql.Result
	err        error
	query      string
	args       []any
	execCalled bool
}

func (db *fakeSummarySQL) QueryRowContext(_ context.Context, query string, args ...any) RowScanner {
	db.query = query
	db.args = args
	return db.row
}

func (db *fakeSummarySQL) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.execCalled = true
	db.query = query
	db.args = args
	return db.result, db.err
}

type fakeSummaryRow struct {
	summary SessionSummary
	err     error
}

func (r fakeSummaryRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*string)) = r.summary.SessionID
	*(dest[1].(*string)) = r.summary.UserID
	*(dest[2].(*string)) = r.summary.Content
	*(dest[3].(*int64)) = r.summary.LastMessageID
	*(dest[4].(*int64)) = r.summary.Version
	*(dest[5].(*time.Time)) = r.summary.CreatedAt
	*(dest[6].(*time.Time)) = r.summary.UpdatedAt
	return nil
}

type fakeSQLResult struct {
	affected int64
	err      error
}

func (r fakeSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeSQLResult) RowsAffected() (int64, error) { return r.affected, r.err }

func TestMySQLSummaryQueriesFind(t *testing.T) {
	want := SessionSummary{SessionID: "s", UserID: "u", Content: "summary", LastMessageID: 20, Version: 2}
	db := &fakeSummarySQL{row: fakeSummaryRow{summary: want}}

	got, err := newMySQLSummaryQueries(db).Find("s", "u")
	if err != nil || got != want {
		t.Fatalf("Find()=(%+v,%v), want (%+v,nil)", got, err, want)
	}
	if !strings.Contains(db.query, "FROM session_summaries") || len(db.args) != 2 {
		t.Fatalf("Find() query=%q args=%#v", db.query, db.args)
	}
}

func TestMySQLSummaryQueriesInsertAndUpdateUseVersionGuards(t *testing.T) {
	next := SessionSummary{SessionID: "s", UserID: "u", Content: "next", LastMessageID: 24, Version: 3}

	insertDB := &fakeSummarySQL{result: fakeSQLResult{affected: 1}}
	affected, err := newMySQLSummaryQueries(insertDB).Insert(next)
	if err != nil || affected != 1 || !strings.Contains(insertDB.query, "INSERT INTO session_summaries") {
		t.Fatalf("Insert() affected=%d error=%v query=%q", affected, err, insertDB.query)
	}

	updateDB := &fakeSummarySQL{result: fakeSQLResult{affected: 1}}
	affected, err = newMySQLSummaryQueries(updateDB).Update(next, 2)
	if err != nil || affected != 1 {
		t.Fatalf("Update() affected=%d error=%v", affected, err)
	}
	if !strings.Contains(updateDB.query, "version = ?") || !strings.Contains(updateDB.query, "last_message_id <= ?") {
		t.Fatalf("Update() lacks guards: %q", updateDB.query)
	}
	if len(updateDB.args) < 2 || updateDB.args[len(updateDB.args)-2] != int64(2) || updateDB.args[len(updateDB.args)-1] != int64(24) {
		t.Fatalf("Update() args=%#v, want expected version and watermark guards last", updateDB.args)
	}
}

func TestMySQLSummaryQueriesPropagatesExecAndRowsAffectedErrors(t *testing.T) {
	execErr := errors.New("exec failed")
	if _, err := newMySQLSummaryQueries(&fakeSummarySQL{err: execErr}).Insert(SessionSummary{}); !errors.Is(err, execErr) {
		t.Fatalf("Insert() error=%v, want exec error", err)
	}
	rowsErr := errors.New("rows failed")
	if _, err := newMySQLSummaryQueries(&fakeSummarySQL{result: fakeSQLResult{err: rowsErr}}).Update(SessionSummary{}, 1); !errors.Is(err, rowsErr) {
		t.Fatalf("Update() error=%v, want rows error", err)
	}
}
