package recentchat

type RecentWindowBuilder interface {
	Build(messages []Message, maxMessages int) []Message
}
