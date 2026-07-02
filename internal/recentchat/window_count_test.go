package recentchat

import "testing"

func TestCountWindowBuilderKeepsLatestMessagesInChronologicalOrder(t *testing.T) {
	builder := CountWindowBuilder{}
	in := []Message{
		{Content: "1"},
		{Content: "2"},
		{Content: "3"},
		{Content: "4"},
	}

	out := builder.Build(in, 2)
	if len(out) != 2 || out[0].Content != "3" || out[1].Content != "4" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestCountWindowBuilderReturnsAllMessagesWhenLimitExceedsInput(t *testing.T) {
	builder := CountWindowBuilder{}
	in := []Message{
		{Content: "1"},
		{Content: "2"},
	}

	out := builder.Build(in, 5)
	if len(out) != 2 || out[0].Content != "1" || out[1].Content != "2" {
		t.Fatalf("unexpected output: %#v", out)
	}
}
