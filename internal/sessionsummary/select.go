package sessionsummary

import "fmt"

type MessageTokenCounter interface {
	CountMessage(message SourceMessage) (int, error)
}

type PrefixSelection struct {
	Unsummarized       []SourceMessage `json:"unsummarized"`
	Evicted            []SourceMessage `json:"evicted"`
	UnsummarizedTokens int             `json:"unsummarized_tokens"`
	EvictedTokens      int             `json:"evicted_tokens"`
	NextWatermark      int64           `json:"next_watermark"`
}

func SelectPrefix(
	messages []SourceMessage,
	lastMessageID int64,
	recentStartID int64,
	counter MessageTokenCounter,
) (PrefixSelection, error) {
	if lastMessageID < 0 {
		return PrefixSelection{}, fmt.Errorf("last message watermark must not be negative: %d", lastMessageID)
	}
	if recentStartID < 0 {
		return PrefixSelection{}, fmt.Errorf("recent start ID must not be negative: %d", recentStartID)
	}
	if counter == nil {
		return PrefixSelection{}, fmt.Errorf("message token counter is required")
	}
	if err := validateSourceMessages(messages); err != nil {
		return PrefixSelection{}, err
	}

	unsummarized := make([]SourceMessage, 0, len(messages))
	for _, message := range messages {
		if message.ID > lastMessageID {
			unsummarized = append(unsummarized, message)
		}
	}

	evictedCount := len(unsummarized)
	if recentStartID > 0 {
		if recentStartID <= lastMessageID {
			// The recent window reaches into already summarized history, so every
			// message after the watermark is still recent and none is evicted.
			evictedCount = 0
		} else {
			evictedCount = -1
			for i, message := range unsummarized {
				if message.ID == recentStartID {
					evictedCount = i
					break
				}
			}
			if evictedCount < 0 {
				return PrefixSelection{}, fmt.Errorf("recent start ID %d not found after watermark %d", recentStartID, lastMessageID)
			}
		}
	}

	selection := PrefixSelection{
		Unsummarized:  unsummarized,
		Evicted:       append([]SourceMessage(nil), unsummarized[:evictedCount]...),
		NextWatermark: lastMessageID,
	}
	for i, message := range unsummarized {
		count, err := counter.CountMessage(message)
		if err != nil {
			return PrefixSelection{}, fmt.Errorf("count message %d tokens: %w", message.ID, err)
		}
		selection.UnsummarizedTokens += count
		if i < evictedCount {
			selection.EvictedTokens += count
		}
	}
	if evictedCount > 0 {
		selection.NextWatermark = unsummarized[evictedCount-1].ID
	}
	return selection, nil
}

func validateSourceMessages(messages []SourceMessage) error {
	var previousID int64
	for i, message := range messages {
		if message.ID <= 0 {
			return fmt.Errorf("message %d ID must be positive: %d", i, message.ID)
		}
		if i > 0 && message.ID <= previousID {
			return fmt.Errorf("message IDs must be strictly increasing: %d then %d", previousID, message.ID)
		}
		previousID = message.ID
	}
	return nil
}
