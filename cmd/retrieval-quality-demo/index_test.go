package main

import (
	"context"
	"fmt"
	"testing"

	"offline-rag-go-lab/internal/retrievalquality"
)

type recordingIndex struct {
	events  []string
	upserts int
}

func (r *recordingIndex) EnsureCollection(_ context.Context, size int) error {
	r.events = append(r.events, fmt.Sprintf("ensure:%d", size))
	return nil
}

func (r *recordingIndex) Upsert(_ context.Context, _ retrievalquality.IndexedChunk) error {
	r.upserts++
	return nil
}

func (r *recordingIndex) Inspect(_ context.Context, size, points int) (retrievalquality.CollectionState, error) {
	r.events = append(r.events, fmt.Sprintf("inspect:%d:%d", size, points))
	return retrievalquality.CollectionState{Status: "green", PointCount: points, VectorSize: size}, nil
}

func (r *recordingIndex) Activate(_ context.Context) error {
	r.events = append(r.events, "activate")
	return nil
}

func TestIndexDatasetVerifiesBeforeAliasActivation(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	tokenizer := tokenMapForCommand{}
	index := &recordingIndex{}
	report, err := indexDataset(context.Background(), dataset, index, constantEmbedder{}, tokenizer, "bge-m3", "qwen2:test", true)
	if err != nil {
		t.Fatal(err)
	}
	if index.upserts != 24 || report.PointCount != 24 || !report.AliasActivated {
		t.Fatalf("upserts=%d report=%+v", index.upserts, report)
	}
	wantEvents := []string{"ensure:2", "inspect:2:24", "activate"}
	if fmt.Sprint(index.events) != fmt.Sprint(wantEvents) {
		t.Fatalf("events=%v, want %v", index.events, wantEvents)
	}
}

type tokenMapForCommand struct{}

func (tokenMapForCommand) TokenIDs(text string) ([]int, error) {
	if text == "" {
		return []int{}, nil
	}
	return []int{len([]rune(text))}, nil
}
