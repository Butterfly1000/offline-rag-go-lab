package sessionsummary

import (
	"context"
	"database/sql"
	"fmt"
)

type MessageRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type MessageSQL interface {
	QueryContext(ctx context.Context, query string, args ...any) (MessageRows, error)
}

type mysqlMessageSource struct {
	db MessageSQL
}

func newMySQLMessageSource(db MessageSQL) *mysqlMessageSource {
	return &mysqlMessageSource{db: db}
}

// NewMySQLMessageSource reads only messages newer than the committed summary
// watermark. Ordering by the primary key makes prefix selection deterministic.
func NewMySQLMessageSource(db *sql.DB) MessageSource {
	return newMySQLMessageSource(messageSQLAdapter{db: db})
}

func (s *mysqlMessageSource) ListAfter(sessionID, userID string, lastMessageID int64) ([]SourceMessage, error) {
	if err := validateSummaryKey(sessionID, userID); err != nil {
		return nil, err
	}
	if lastMessageID < 0 {
		return nil, fmt.Errorf("last message watermark must not be negative: %d", lastMessageID)
	}
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("message SQL is required")
	}

	rows, err := s.db.QueryContext(
		context.Background(),
		`SELECT id, role, content
		FROM recent_chat_messages
		WHERE session_id = ? AND user_id = ? AND id > ?
		ORDER BY id ASC`,
		sessionID,
		userID,
		lastMessageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]SourceMessage, 0)
	for rows.Next() {
		var message SourceMessage
		if err := rows.Scan(&message.ID, &message.Role, &message.Content); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messages, nil
}

type messageSQLAdapter struct {
	db *sql.DB
}

func (a messageSQLAdapter) QueryContext(ctx context.Context, query string, args ...any) (MessageRows, error) {
	if a.db == nil {
		return nil, fmt.Errorf("MySQL database is required")
	}
	return a.db.QueryContext(ctx, query, args...)
}

var _ MessageSource = (*mysqlMessageSource)(nil)
