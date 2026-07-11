package sessionsummary

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type fakeSummaryQueries struct {
	found           SessionSummary
	findErr         error
	insertAffected  int64
	insertErr       error
	updateAffected  int64
	updateErr       error
	inserted        SessionSummary
	updated         SessionSummary
	expectedVersion int64
}

func (q *fakeSummaryQueries) Find(sessionID, userID string) (SessionSummary, error) {
	return q.found, q.findErr
}

func (q *fakeSummaryQueries) Insert(next SessionSummary) (int64, error) {
	q.inserted = next
	return q.insertAffected, q.insertErr
}

func (q *fakeSummaryQueries) Update(next SessionSummary, expectedVersion int64) (int64, error) {
	q.updated = next
	q.expectedVersion = expectedVersion
	return q.updateAffected, q.updateErr
}

func TestStoreGetReturnsMissingAndExistingSummary(t *testing.T) {
	missing := NewStore(&fakeSummaryQueries{findErr: sql.ErrNoRows})
	if _, ok, err := missing.Get("s-001", "u-001"); err != nil || ok {
		t.Fatalf("Get() ok=%v error=%v, want missing without error", ok, err)
	}

	want := SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "summary", LastMessageID: 20, Version: 2}
	existing := NewStore(&fakeSummaryQueries{found: want})
	got, ok, err := existing.Get("s-001", "u-001")
	if err != nil || !ok || got != want {
		t.Fatalf("Get()=(%+v,%v,%v), want (%+v,true,nil)", got, ok, err, want)
	}
}

func TestStoreSaveInsertsFirstVersion(t *testing.T) {
	queries := &fakeSummaryQueries{insertAffected: 1}
	store := NewStore(queries)

	got, err := store.Save(SessionSummary{
		SessionID: "s-001", UserID: "u-001", Content: "first", LastMessageID: 20,
	}, 0)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got.Version != 1 || got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("Save() = %+v, want version 1 and timestamps", got)
	}
	if queries.inserted != got {
		t.Fatalf("Insert() = %+v, want returned summary %+v", queries.inserted, got)
	}
}

func TestStoreSaveUpdatesMatchingVersion(t *testing.T) {
	createdAt := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	queries := &fakeSummaryQueries{
		found:          SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "old", LastMessageID: 20, Version: 2, CreatedAt: createdAt},
		updateAffected: 1,
	}

	got, err := NewStore(queries).Save(SessionSummary{
		SessionID: "s-001", UserID: "u-001", Content: "next", LastMessageID: 24,
	}, 2)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got.Version != 3 || got.CreatedAt != createdAt || got.UpdatedAt.IsZero() {
		t.Fatalf("Save() = %+v, want version 3 with preserved created_at", got)
	}
	if queries.expectedVersion != 2 || queries.updated != got {
		t.Fatalf("Update() summary=%+v expectedVersion=%d", queries.updated, queries.expectedVersion)
	}
}

func TestStoreSaveRejectsConflictAndWatermarkRegression(t *testing.T) {
	current := SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "old", LastMessageID: 20, Version: 2}

	staleQueries := &fakeSummaryQueries{found: current, updateAffected: 1}
	_, err := NewStore(staleQueries).Save(SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "next", LastMessageID: 24}, 1)
	if !errors.Is(err, ErrVersionConflict) || staleQueries.updated.SessionID != "" {
		t.Fatalf("Save() error=%v updated=%+v, want pre-write version conflict", err, staleQueries.updated)
	}

	regressionQueries := &fakeSummaryQueries{found: current, updateAffected: 1}
	_, err = NewStore(regressionQueries).Save(SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "next", LastMessageID: 19}, 2)
	if !errors.Is(err, ErrWatermarkRegression) || regressionQueries.updated.SessionID != "" {
		t.Fatalf("Save() error=%v updated=%+v, want pre-write watermark error", err, regressionQueries.updated)
	}

	racedQueries := &fakeSummaryQueries{found: current, updateAffected: 0}
	_, err = NewStore(racedQueries).Save(SessionSummary{SessionID: "s-001", UserID: "u-001", Content: "next", LastMessageID: 24}, 2)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Save() error=%v, want raced version conflict", err)
	}
}

func TestStoreValidatesInputAndPropagatesQueryErrors(t *testing.T) {
	store := NewStore(&fakeSummaryQueries{})
	if _, _, err := store.Get("", "u-001"); err == nil {
		t.Fatal("Get() error=nil, want session validation error")
	}
	invalid := []struct {
		name     string
		next     SessionSummary
		expected int64
	}{
		{name: "empty content", next: SessionSummary{SessionID: "s", UserID: "u", LastMessageID: 1}},
		{name: "zero watermark", next: SessionSummary{SessionID: "s", UserID: "u", Content: "x"}},
		{name: "negative version", next: SessionSummary{SessionID: "s", UserID: "u", Content: "x", LastMessageID: 1}, expected: -1},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.Save(test.next, test.expected); err == nil {
				t.Fatal("Save() error=nil, want validation error")
			}
		})
	}

	findFailure := NewStore(&fakeSummaryQueries{findErr: errors.New("db down")})
	if _, _, err := findFailure.Get("s", "u"); err == nil {
		t.Fatal("Get() error=nil, want query error")
	}

	duplicate := NewStore(&fakeSummaryQueries{insertErr: &mysql.MySQLError{Number: 1062, Message: "duplicate"}})
	_, err := duplicate.Save(SessionSummary{SessionID: "s", UserID: "u", Content: "x", LastMessageID: 1}, 0)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Save() error=%v, want duplicate as version conflict", err)
	}
}
