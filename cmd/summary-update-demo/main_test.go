package main

import (
	"testing"

	"offline-rag-go-lab/internal/sessionsummary"
)

func TestChooseRecentStartID(t *testing.T) {
	messages := []sessionsummary.SourceMessage{{ID: 10}, {ID: 11}, {ID: 15}, {ID: 20}}
	tests := []struct {
		keep int
		want int64
	}{
		{keep: 0, want: 0},
		{keep: 2, want: 15},
		{keep: 4, want: 10},
		{keep: 10, want: 10},
	}
	for _, test := range tests {
		got, err := chooseRecentStartID(messages, test.keep)
		if err != nil || got != test.want {
			t.Fatalf("chooseRecentStartID(keep=%d)=(%d,%v), want (%d,nil)", test.keep, got, err, test.want)
		}
	}
	if _, err := chooseRecentStartID(messages, -1); err == nil {
		t.Fatal("chooseRecentStartID() error=nil, want negative keep error")
	}
}
