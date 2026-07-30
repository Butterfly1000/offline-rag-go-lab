package main

import (
	"context"
	"testing"

	"offline-rag-go-lab/internal/retrievalquality"
)

type constantEmbedder struct{}

func (constantEmbedder) Embed(_ context.Context, _ string, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = []float32{1, 0}
	}
	return vectors, nil
}

func TestBuildDatasetReportCoversFixedSplitsAndSafety(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	output, err := buildDatasetReport(context.Background(), dataset, constantEmbedder{}, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	if output.CaseCount != 40 || output.Train.CaseCount != 24 || output.Validation.CaseCount != 16 {
		t.Fatalf("case counts total=%d train=%d validation=%d", output.CaseCount, output.Train.CaseCount, output.Validation.CaseCount)
	}
	if output.Train.ScopeIsolation != 1 || output.Validation.ScopeIsolation != 1 {
		t.Fatalf("scope isolation train=%v validation=%v", output.Train.ScopeIsolation, output.Validation.ScopeIsolation)
	}
	if output.Train.ForbiddenHitCount != 0 || output.Validation.ForbiddenHitCount != 0 {
		t.Fatalf("forbidden hits train=%d validation=%d", output.Train.ForbiddenHitCount, output.Validation.ForbiddenHitCount)
	}
}
