package main

import (
	"context"
	"testing"

	"offline-rag-go-lab/internal/retrievalquality"
)

func TestBuildHybridReportComparesThreeStrategiesAndKeepsTrace(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	dense := safeSparseStrategy{corpus: dataset.Corpus}
	sparse := safeSparseStrategy{corpus: dataset.Corpus}
	hybrid, err := retrievalquality.NewHybridStrategy(
		dense, sparse, retrievalquality.FusionWeights{Dense: 1, Sparse: 1},
		retrievalquality.HybridEvaluation,
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := buildHybridReport(context.Background(), dataset, dense, sparse, hybrid)
	if err != nil {
		t.Fatal(err)
	}
	if report.Dense.Validation.CaseCount != 16 || report.Sparse.Validation.CaseCount != 16 ||
		report.Hybrid.Validation.CaseCount != 16 {
		t.Fatalf("report counts=%+v", report)
	}
	if len(report.TraceExample.Candidates) == 0 {
		t.Fatal("trace example is empty")
	}
	trace := report.TraceExample.Candidates[0].Trace
	if trace["dense_rank"] != 1 || trace["sparse_rank"] != 1 ||
		trace["dense_contribution"] == 0 || trace["sparse_contribution"] == 0 {
		t.Fatalf("trace=%v", trace)
	}
}
