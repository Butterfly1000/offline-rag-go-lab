package memoryitem

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

type RowScanner interface {
	Scan(dest ...any) error
}

type RowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type MemorySQL interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
	QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type mysqlMemoryQueries struct {
	db MemorySQL
}

func newMySQLMemoryQueries(db MemorySQL) *mysqlMemoryQueries {
	return &mysqlMemoryQueries{db: db}
}

func (q *mysqlMemoryQueries) Find(ctx context.Context, userID string, kind Kind, key string) (Item, error) {
	return q.find(ctx, userID, kind, key, false)
}

func (q *mysqlMemoryQueries) FindForUpdate(ctx context.Context, userID string, kind Kind, key string) (Item, error) {
	return q.find(ctx, userID, kind, key, true)
}

func (q *mysqlMemoryQueries) find(ctx context.Context, userID string, kind Kind, key string, lock bool) (Item, error) {
	query := `SELECT id, user_id, kind, memory_key, value, status, version, created_at, updated_at
		FROM memory_items
		WHERE user_id = ? AND kind = ? AND memory_key = ?`
	if lock {
		query += " FOR UPDATE"
	}
	var item Item
	err := q.db.QueryRowContext(ctx, query, userID, kind, key).Scan(
		&item.ID, &item.UserID, &item.Kind, &item.Key, &item.Value,
		&item.Status, &item.Version, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (q *mysqlMemoryQueries) ListActive(ctx context.Context, userID string) ([]Item, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, user_id, kind, memory_key, value, status, version, created_at, updated_at
		FROM memory_items
		WHERE user_id = ? AND status = 'active'
		ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Kind, &item.Key, &item.Value,
			&item.Status, &item.Version, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *mysqlMemoryQueries) InsertItem(ctx context.Context, item Item) (int64, error) {
	result, err := q.db.ExecContext(ctx, `INSERT INTO memory_items
		(user_id, kind, memory_key, value, status, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.UserID, item.Kind, item.Key, item.Value, item.Status, item.Version, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, fmt.Errorf("SQL result is required")
	}
	return result.LastInsertId()
}

func (q *mysqlMemoryQueries) UpdateItem(ctx context.Context, item Item, expectedVersion int64) (int64, error) {
	result, err := q.db.ExecContext(ctx, `UPDATE memory_items
		SET value = ?, status = ?, version = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND kind = ? AND memory_key = ? AND version = ?`,
		item.Value, item.Status, item.Version, item.UpdatedAt,
		item.ID, item.UserID, item.Kind, item.Key, expectedVersion,
	)
	return memoryRowsAffected(result, err)
}

func (q *mysqlMemoryQueries) InsertEvidence(ctx context.Context, evidence Evidence) (int64, error) {
	result, err := q.db.ExecContext(ctx, `INSERT INTO memory_item_evidence
		(memory_item_id, user_id, source_session_id, source_message_id, source_role, operation, evidence_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.ItemID, evidence.UserID, evidence.SessionID, evidence.MessageID,
		evidence.Role, evidence.Operation, evidence.Text, evidence.CreatedAt,
	)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return 0, nil
		}
		return 0, err
	}
	return memoryRowsAffected(result, nil)
}

func memoryRowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	if result == nil {
		return 0, fmt.Errorf("SQL result is required")
	}
	return result.RowsAffected()
}

type mysqlMemoryTransactionFactory struct {
	db *sql.DB
}

func (f mysqlMemoryTransactionFactory) Begin(ctx context.Context) (MemoryUnitOfWork, error) {
	if f.db == nil {
		return nil, fmt.Errorf("MySQL database is required")
	}
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &mysqlMemoryUnitOfWork{tx: tx, queries: newMySQLMemoryQueries(sqlTxMemoryAdapter{tx: tx})}, nil
}

type mysqlMemoryUnitOfWork struct {
	tx      *sql.Tx
	queries *mysqlMemoryQueries
}

func (u *mysqlMemoryUnitOfWork) FindForUpdate(ctx context.Context, userID string, kind Kind, key string) (Item, error) {
	return u.queries.FindForUpdate(ctx, userID, kind, key)
}

func (u *mysqlMemoryUnitOfWork) InsertItem(ctx context.Context, item Item) (int64, error) {
	return u.queries.InsertItem(ctx, item)
}

func (u *mysqlMemoryUnitOfWork) UpdateItem(ctx context.Context, item Item, expectedVersion int64) (int64, error) {
	return u.queries.UpdateItem(ctx, item, expectedVersion)
}

func (u *mysqlMemoryUnitOfWork) InsertEvidence(ctx context.Context, evidence Evidence) (int64, error) {
	return u.queries.InsertEvidence(ctx, evidence)
}

func (u *mysqlMemoryUnitOfWork) Commit() error   { return u.tx.Commit() }
func (u *mysqlMemoryUnitOfWork) Rollback() error { return u.tx.Rollback() }

func NewMySQLMemoryStore(db *sql.DB) MemoryStore {
	return NewStore(
		newMySQLMemoryQueries(sqlDBMemoryAdapter{db: db}),
		mysqlMemoryTransactionFactory{db: db},
	)
}

type sqlDBMemoryAdapter struct {
	db *sql.DB
}

func (a sqlDBMemoryAdapter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	if a.db == nil {
		return memoryErrorRow{err: fmt.Errorf("MySQL database is required")}
	}
	return a.db.QueryRowContext(ctx, query, args...)
}

func (a sqlDBMemoryAdapter) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	if a.db == nil {
		return nil, fmt.Errorf("MySQL database is required")
	}
	return a.db.QueryContext(ctx, query, args...)
}

func (a sqlDBMemoryAdapter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if a.db == nil {
		return nil, fmt.Errorf("MySQL database is required")
	}
	return a.db.ExecContext(ctx, query, args...)
}

type sqlTxMemoryAdapter struct {
	tx *sql.Tx
}

func (a sqlTxMemoryAdapter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return a.tx.QueryRowContext(ctx, query, args...)
}

func (a sqlTxMemoryAdapter) QueryContext(ctx context.Context, query string, args ...any) (RowsScanner, error) {
	return a.tx.QueryContext(ctx, query, args...)
}

func (a sqlTxMemoryAdapter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return a.tx.ExecContext(ctx, query, args...)
}

type memoryErrorRow struct {
	err error
}

func (r memoryErrorRow) Scan(...any) error { return r.err }

var (
	_ MemoryReader             = (*mysqlMemoryQueries)(nil)
	_ MemoryUnitOfWork         = (*mysqlMemoryUnitOfWork)(nil)
	_ MemoryTransactionFactory = mysqlMemoryTransactionFactory{}
)
