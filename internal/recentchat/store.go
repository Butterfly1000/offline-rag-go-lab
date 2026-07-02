package recentchat

type MessageStore interface {
	ListRecentBySession(sessionID string, limit int) ([]Message, error)
	Append(msg Message) error
}
