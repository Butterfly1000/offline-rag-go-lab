package main

import (
	"context"
	"testing"

	"offline-rag-go-lab/internal/retrievalquality"
)

type corpusMetadataStrategy struct {
	corpus []retrievalquality.Chunk
}

func (corpusMetadataStrategy) Name() string { return "hybrid_test" }

func (s corpusMetadataStrategy) Search(_ context.Context, query retrievalquality.Query) (retrievalquality.SearchResult, error) {
	result := retrievalquality.SearchResult{}
	for _, chunk := range s.corpus {
		if chunk.KnowledgeScope != query.KnowledgeScope {
			continue
		}
		result.Candidates = append(result.Candidates, retrievalquality.Candidate{
			KnowledgeScope: chunk.KnowledgeScope,
			DocumentID:     chunk.DocumentID,
			ChunkID:        chunk.ChunkID,
			HeadingPath:    chunk.HeadingPath,
			Content:        chunk.Content,
			Score:          float64(len(s.corpus) - len(result.Candidates)),
		})
		if len(result.Candidates) == query.Limit {
			break
		}
	}
	return result, nil
}

type preserveOrderReranker struct{}

func (preserveOrderReranker) Rank(
	_ context.Context, _ string, candidates []retrievalquality.Candidate,
) ([]retrievalquality.RerankScore, error) {
	result := make([]retrievalquality.RerankScore, len(candidates))
	for index, candidate := range candidates {
		result[index] = retrievalquality.RerankScore{
			CandidateID: candidate.ChunkID,
			Relevance:   float64(len(candidates) - index),
		}
	}
	return result, nil
}

func TestBuildRerankReportUsesValidationAndDoesNotEnableWithoutImprovement(t *testing.T) {
	dataset, err := retrievalquality.LoadDataset("../../internal/retrievalquality/testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	base := corpusMetadataStrategy{corpus: dataset.Corpus}
	reranked, err := retrievalquality.NewRerankedStrategy(
		base, preserveOrderReranker{},
		retrievalquality.DiversityLimits{PerDocument: 3, PerHeading: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := buildRerankReport(context.Background(), dataset, base, reranked, "test-model")
	if err != nil {
		t.Fatal(err)
	}
	if report.HybridValidation.CaseCount != 16 || report.RerankedValidation.CaseCount != 16 {
		t.Fatalf("case counts hybrid=%d reranked=%d", report.HybridValidation.CaseCount, report.RerankedValidation.CaseCount)
	}
	if report.Stats.Calls != 16 || report.Stats.Used != 16 || report.Stats.Fallbacks != 0 {
		t.Fatalf("stats=%+v", report.Stats)
	}
	if report.DefaultEnabled {
		t.Fatalf("reranker must stay disabled when NDCG does not improve: %+v", report)
	}
	if !report.Passed {
		t.Fatalf("report should pass safety gates: %+v", report)
	}
}

func TestSameCandidateOrderComparesIdentityOnly(t *testing.T) {
	left := []retrievalquality.Candidate{{ChunkID: "a", Score: 1}, {ChunkID: "b", Score: 0.5}}
	right := []retrievalquality.Candidate{{ChunkID: "a", Score: 99}, {ChunkID: "b", Score: -1}}
	if !sameCandidateOrder(left, right) {
		t.Fatal("same IDs in the same order should match")
	}
	right[1].ChunkID = "c"
	if sameCandidateOrder(left, right) {
		t.Fatal("different IDs should not match")
	}
}
