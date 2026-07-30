package main

import (
	"context"
	"testing"

	"offline-rag-go-lab/internal/retrievalquality"
)

type safeSparseStrategy struct {
	corpus []retrievalquality.Chunk
}

func (safeSparseStrategy) Name() string { return "qdrant_sparse_test" }

func (s safeSparseStrategy) Search(_ context.Context, query retrievalquality.Query) (retrievalquality.SearchResult, error) {
	result := retrievalquality.SearchResult{}
	for _, chunk := range s.corpus {
		if chunk.KnowledgeScope != query.KnowledgeScope {
			continue
		}
		result.Candidates = append(result.Candidates, retrievalquality.Candidate{
			KnowledgeScope: chunk.KnowledgeScope, ChunkID: chunk.ChunkID,
		})
		if len(result.Candidates) == query.Limit {
			break
		}
	}
	return result, nil
}

func TestBuildSparseReportUsesFixedDatasetSplits(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	report, err := buildSparseReport(context.Background(), dataset, safeSparseStrategy{corpus: dataset.Corpus})
	if err != nil {
		t.Fatal(err)
	}
	if report.Train.CaseCount != 24 || report.Validation.CaseCount != 16 {
		t.Fatalf("counts train=%d validation=%d", report.Train.CaseCount, report.Validation.CaseCount)
	}
	if !report.Passed || report.Train.ForbiddenHitCount != 0 || report.Validation.ForbiddenHitCount != 0 {
		t.Fatalf("unsafe report=%+v", report)
	}
}
