package sessionsummary

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeMessageSQL struct {
	rows  MessageRows
	err   error
	query string
	args  []any
}

func (db *fakeMessageSQL) QueryContext(_ context.Context, query string, args ...any) (MessageRows, error) {
	db.query = query
	db.args = args
	return db.rows, db.err
}

type fakeMessageRows struct {
	messages []SourceMessage
	index    int
	scanErr  error
	rowsErr  error
	closed   bool
}

func (r *fakeMessageRows) Next() bool {
	return r.index < len(r.messages)
}

func (r *fakeMessageRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	message := r.messages[r.index]
	r.index++
	*(dest[0].(*int64)) = message.ID
	*(dest[1].(*string)) = message.Role
	*(dest[2].(*string)) = message.Content
	return nil
}

func (r *fakeMessageRows) Err() error {
	return r.rowsErr
}

func (r *fakeMessageRows) Close() error {
	r.closed = true
	return nil
}

func TestMySQLMessageSourceListsAscendingMessagesAfterWatermark(t *testing.T) {
	rows := &fakeMessageRows{messages: []SourceMessage{
		{ID: 21, Role: "user", Content: "one"},
		{ID: 23, Role: "assistant", Content: "two"},
	}}
	db := &fakeMessageSQL{rows: rows}

	got, err := newMySQLMessageSource(db).ListAfter("s", "u", 20)
	if err != nil {
		t.Fatalf("ListAfter() error = %v", err)
	}
	assertMessageIDs(t, got, 21, 23)
	if !rows.closed {
		t.Fatal("ListAfter() did not close rows")
	}
	if !strings.Contains(db.query, "id > ?") || !strings.Contains(db.query, "ORDER BY id ASC") {
		t.Fatalf("ListAfter() query = %q", db.query)
	}
	if len(db.args) != 3 || db.args[0] != "s" || db.args[1] != "u" || db.args[2] != int64(20) {
		t.Fatalf("ListAfter() args = %#v", db.args)
	}
}

func TestMySQLMessageSourcePropagatesQueryScanAndRowsErrors(t *testing.T) {
	queryErr := errors.New("query failed")
	if _, err := newMySQLMessageSource(&fakeMessageSQL{err: queryErr}).ListAfter("s", "u", 0); !errors.Is(err, queryErr) {
		t.Fatalf("ListAfter() error=%v, want query error", err)
	}

	scanErr := errors.New("scan failed")
	if _, err := newMySQLMessageSource(&fakeMessageSQL{rows: &fakeMessageRows{messages: sourceMessages(1), scanErr: scanErr}}).ListAfter("s", "u", 0); !errors.Is(err, scanErr) {
		t.Fatalf("ListAfter() error=%v, want scan error", err)
	}

	rowsErr := errors.New("rows failed")
	if _, err := newMySQLMessageSource(&fakeMessageSQL{rows: &fakeMessageRows{rowsErr: rowsErr}}).ListAfter("s", "u", 0); !errors.Is(err, rowsErr) {
		t.Fatalf("ListAfter() error=%v, want rows error", err)
	}
}
