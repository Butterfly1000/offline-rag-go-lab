package recentchat

type MessageStore interface {
	ListRecentBySessionUser(sessionID, userID string, limit int) ([]Message, error)
	Append(msg Message) error
}
