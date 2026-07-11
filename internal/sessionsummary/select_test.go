package sessionsummary

import (
	"errors"
	"strings"
	"testing"
)

type fakeMessageTokenCounter struct {
	counts map[int64]int
	errAt  int64
}

func (c fakeMessageTokenCounter) CountMessage(message SourceMessage) (int, error) {
	if message.ID == c.errAt {
		return 0, errors.New("count failed")
	}
	return c.counts[message.ID], nil
}

func TestSelectPrefixKeepsRecentAndSelectsEvictedPrefix(t *testing.T) {
	messages := sourceMessages(19, 20, 21, 22, 23, 24, 25, 26)
	counter := fakeMessageTokenCounter{counts: map[int64]int{
		21: 2, 22: 3, 23: 4, 24: 5, 25: 6, 26: 7,
	}}

	got, err := SelectPrefix(messages, 20, 25, counter)
	if err != nil {
		t.Fatalf("SelectPrefix() error = %v", err)
	}
	assertMessageIDs(t, got.Unsummarized, 21, 22, 23, 24, 25, 26)
	assertMessageIDs(t, got.Evicted, 21, 22, 23, 24)
	if got.UnsummarizedTokens != 27 || got.EvictedTokens != 14 {
		t.Fatalf("SelectPrefix() tokens = unsummarized %d evicted %d", got.UnsummarizedTokens, got.EvictedTokens)
	}
	if got.NextWatermark != 24 {
		t.Fatalf("SelectPrefix() next watermark = %d, want 24", got.NextWatermark)
	}
}

func TestSelectPrefixEvictsAllWhenRecentWindowIsEmpty(t *testing.T) {
	got, err := SelectPrefix(
		sourceMessages(21, 23, 26),
		20,
		0,
		fakeMessageTokenCounter{counts: map[int64]int{21: 2, 23: 3, 26: 4}},
	)
	if err != nil {
		t.Fatalf("SelectPrefix() error = %v", err)
	}
	assertMessageIDs(t, got.Evicted, 21, 23, 26)
	if got.NextWatermark != 26 || got.EvictedTokens != 9 {
		t.Fatalf("SelectPrefix() = %+v", got)
	}
}

func TestSelectPrefixDoesNotAdvanceWithoutEviction(t *testing.T) {
	got, err := SelectPrefix(
		sourceMessages(21, 22),
		20,
		21,
		fakeMessageTokenCounter{counts: map[int64]int{21: 2, 22: 3}},
	)
	if err != nil {
		t.Fatalf("SelectPrefix() error = %v", err)
	}
	if len(got.Evicted) != 0 || got.NextWatermark != 20 || got.EvictedTokens != 0 {
		t.Fatalf("SelectPrefix() = %+v, want no eviction", got)
	}
}

func TestSelectPrefixTreatsRecentStartBeforeWatermarkAsNoEviction(t *testing.T) {
	got, err := SelectPrefix(
		sourceMessages(21, 22),
		20,
		19,
		fakeMessageTokenCounter{counts: map[int64]int{21: 2, 22: 3}},
	)
	if err != nil {
		t.Fatalf("SelectPrefix() error = %v", err)
	}
	assertMessageIDs(t, got.Unsummarized, 21, 22)
	if len(got.Evicted) != 0 || got.NextWatermark != 20 {
		t.Fatalf("SelectPrefix() = %+v, want all new messages kept in recent window", got)
	}
}

func TestSelectPrefixAllowsMessageIDGaps(t *testing.T) {
	got, err := SelectPrefix(
		sourceMessages(21, 23, 30),
		20,
		30,
		fakeMessageTokenCounter{counts: map[int64]int{21: 1, 23: 1, 30: 1}},
	)
	if err != nil {
		t.Fatalf("SelectPrefix() error = %v", err)
	}
	assertMessageIDs(t, got.Evicted, 21, 23)
	if got.NextWatermark != 23 {
		t.Fatalf("SelectPrefix() next watermark = %d, want 23", got.NextWatermark)
	}
}

func TestSelectPrefixRejectsInvalidBoundariesAndIDs(t *testing.T) {
	tests := []struct {
		name        string
		messages    []SourceMessage
		watermark   int64
		recentStart int64
		want        string
	}{
		{name: "negative watermark", messages: sourceMessages(1), watermark: -1, want: "watermark"},
		{name: "negative recent start", messages: sourceMessages(1), recentStart: -1, want: "recent start"},
		{name: "zero message id", messages: []SourceMessage{{ID: 0}}, want: "positive"},
		{name: "unordered ids", messages: sourceMessages(2, 1), want: "strictly increasing"},
		{name: "duplicate ids", messages: sourceMessages(1, 1), want: "strictly increasing"},
		{name: "missing recent start", messages: sourceMessages(21, 22), watermark: 20, recentStart: 23, want: "not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := SelectPrefix(test.messages, test.watermark, test.recentStart, fakeMessageTokenCounter{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SelectPrefix() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSelectPrefixPropagatesCounterError(t *testing.T) {
	_, err := SelectPrefix(sourceMessages(21, 22), 20, 22, fakeMessageTokenCounter{errAt: 21})
	if err == nil || !strings.Contains(err.Error(), "message 21") {
		t.Fatalf("SelectPrefix() error = %v, want message context", err)
	}
}

func sourceMessages(ids ...int64) []SourceMessage {
	messages := make([]SourceMessage, 0, len(ids))
	for _, id := range ids {
		messages = append(messages, SourceMessage{ID: id, Role: "user", Content: "message"})
	}
	return messages
}

func assertMessageIDs(t *testing.T, messages []SourceMessage, want ...int64) {
	t.Helper()
	if len(messages) != len(want) {
		t.Fatalf("message count = %d, want %d: %#v", len(messages), len(want), messages)
	}
	for i := range want {
		if messages[i].ID != want[i] {
			t.Fatalf("message %d ID = %d, want %d", i, messages[i].ID, want[i])
		}
	}
}
