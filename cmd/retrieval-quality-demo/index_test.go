package main

import (
	"context"
	"fmt"
	"math"
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

type sizedEmbedder struct {
	size  int
	value float32
}

func (e sizedEmbedder) Embed(_ context.Context, _ string, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, e.size)
		if e.size > 0 {
			result[i][0] = e.value
		}
	}
	return result, nil
}

func TestIndexDatasetVerifiesBeforeAliasActivation(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	tokenizer := tokenMapForCommand{}
	index := &recordingIndex{}
	report, err := indexDataset(context.Background(), dataset, index, sizedEmbedder{size: 1024, value: 1}, tokenizer, "bge-m3", "qwen2:test", true)
	if err != nil {
		t.Fatal(err)
	}
	if index.upserts != 24 || report.PointCount != 24 || !report.AliasActivated {
		t.Fatalf("upserts=%d report=%+v", index.upserts, report)
	}
	wantEvents := []string{"ensure:1024", "inspect:1024:24", "activate"}
	if fmt.Sprint(index.events) != fmt.Sprint(wantEvents) {
		t.Fatalf("events=%v, want %v", index.events, wantEvents)
	}
}

func TestIndexDatasetRejectsInvalidDenseVectorsBeforeStoreSideEffects(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		embedder retrievalquality.Embedder
	}{
		{name: "wrong dimension", embedder: sizedEmbedder{size: 2, value: 1}},
		{name: "non finite", embedder: sizedEmbedder{size: 1024, value: float32(math.Inf(1))}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := &recordingIndex{}
			if _, err := indexDataset(
				context.Background(), dataset, index, tt.embedder, tokenMapForCommand{},
				"bge-m3", "qwen2:test", true,
			); err == nil {
				t.Fatal("invalid dense vectors must fail")
			}
			if len(index.events) != 0 || index.upserts != 0 {
				t.Fatalf("store side effects events=%v upserts=%d", index.events, index.upserts)
			}
		})
	}
}

type tokenMapForCommand struct{}

func (tokenMapForCommand) TokenIDs(text string) ([]int, error) {
	if text == "" {
		return []int{}, nil
	}
	return []int{len([]rune(text))}, nil
}
