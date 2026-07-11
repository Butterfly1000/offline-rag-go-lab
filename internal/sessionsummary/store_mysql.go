package sessionsummary

import (
	"context"
	"database/sql"
	"fmt"
)

type RowScanner interface {
	Scan(dest ...any) error
}

type SummarySQL interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type mysqlSummaryQueries struct {
	db SummarySQL
}

func newMySQLSummaryQueries(db SummarySQL) *mysqlSummaryQueries {
	return &mysqlSummaryQueries{db: db}
}

// NewMySQLSummaryStore wires the business store to database/sql without exposing SQL upstream.
func NewMySQLSummaryStore(db *sql.DB) SummaryStore {
	return NewStore(newMySQLSummaryQueries(sqlDBAdapter{db: db}))
}

func (q *mysqlSummaryQueries) Find(sessionID, userID string) (SessionSummary, error) {
	var summary SessionSummary
	err := q.db.QueryRowContext(
		context.Background(),
		`SELECT session_id, user_id, content, last_message_id, version, created_at, updated_at
		FROM session_summaries
		WHERE session_id = ? AND user_id = ?`,
		sessionID,
		userID,
	).Scan(
		&summary.SessionID,
		&summary.UserID,
		&summary.Content,
		&summary.LastMessageID,
		&summary.Version,
		&summary.CreatedAt,
		&summary.UpdatedAt,
	)
	return summary, err
}

func (q *mysqlSummaryQueries) Insert(next SessionSummary) (int64, error) {
	result, err := q.db.ExecContext(
		context.Background(),
		`INSERT INTO session_summaries
		(session_id, user_id, content, last_message_id, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		next.SessionID,
		next.UserID,
		next.Content,
		next.LastMessageID,
		next.Version,
		next.CreatedAt,
		next.UpdatedAt,
	)
	return rowsAffected(result, err)
}

func (q *mysqlSummaryQueries) Update(next SessionSummary, expectedVersion int64) (int64, error) {
	// RowsAffected must be exactly one. A concurrent version change or a newer
	// watermark makes this WHERE clause affect zero rows, which Store rejects.
	result, err := q.db.ExecContext(
		context.Background(),
		`UPDATE session_summaries
		SET content = ?, last_message_id = ?, version = ?, updated_at = ?
		WHERE session_id = ? AND user_id = ? AND version = ? AND last_message_id <= ?`,
		next.Content,
		next.LastMessageID,
		next.Version,
		next.UpdatedAt,
		next.SessionID,
		next.UserID,
		expectedVersion,
		next.LastMessageID,
	)
	return rowsAffected(result, err)
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, fmt.Errorf("SQL result is required")
	}
	return result.RowsAffected()
}

type sqlDBAdapter struct {
	db *sql.DB
}

func (a sqlDBAdapter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	if a.db == nil {
		return errorRow{err: fmt.Errorf("MySQL database is required")}
	}
	return a.db.QueryRowContext(ctx, query, args...)
}

func (a sqlDBAdapter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if a.db == nil {
		return nil, fmt.Errorf("MySQL database is required")
	}
	return a.db.ExecContext(ctx, query, args...)
}

type errorRow struct {
	err error
}

func (r errorRow) Scan(...any) error {
	return r.err
}

var _ SummaryQueries = (*mysqlSummaryQueries)(nil)
