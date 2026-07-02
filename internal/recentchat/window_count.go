package recentchat

type CountWindowBuilder struct{}

func (b CountWindowBuilder) Build(messages []Message, maxMessages int) []Message {
	if maxMessages <= 0 || len(messages) <= maxMessages {
		return messages
	}

	return messages[len(messages)-maxMessages:]
}
