package recentchat

import (
	"context"
	"database/sql"
)

type MySQLMessageStore struct {
	db *sql.DB
}

func NewMySQLMessageStore(db *sql.DB) *MySQLMessageStore {
	return &MySQLMessageStore{db: db}
}

func (s *MySQLMessageStore) ListRecentBySession(sessionID string, limit int) ([]Message, error) {
	rows, err := s.db.QueryContext(
		context.Background(),
		`SELECT id, session_id, user_id, role, content, created_at
		FROM recent_chat_messages
		WHERE session_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`,
		sessionID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		if err := rows.Scan(
			&message.ID,
			&message.SessionID,
			&message.UserID,
			&message.Role,
			&message.Content,
			&message.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	return messages, nil
}

func (s *MySQLMessageStore) Append(msg Message) error {
	_, err := s.db.ExecContext(
		context.Background(),
		`INSERT INTO recent_chat_messages (session_id, user_id, role, content, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		msg.SessionID,
		msg.UserID,
		msg.Role,
		msg.Content,
		msg.CreatedAt,
	)
	return err
}
