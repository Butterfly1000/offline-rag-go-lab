package main

import (
	"context"
	"testing"
	"time"

	"offline-rag-go-lab/internal/retrievalquality"
)

func TestOutcomesFromTrainReportExcludesNegativeAndPreservesMetrics(t *testing.T) {
	params := retrievalquality.PolicyParams{
		DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10,
	}
	identity := retrievalquality.EncoderIdentity{
		EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1",
	}
	report := retrievalquality.Report{
		Split: retrievalquality.SplitTrain, LatencyP95: 12 * time.Millisecond,
		ByKind: map[retrievalquality.QueryKind]retrievalquality.KindReport{
			retrievalquality.QueryExact:    {MeanNDCGAt10: 0.9, MeanRecallAt10: 0.8},
			retrievalquality.QueryCode:     {MeanNDCGAt10: 0.7, MeanRecallAt10: 0.6},
			retrievalquality.QuerySemantic: {MeanNDCGAt10: 0.5, MeanRecallAt10: 0.4},
			retrievalquality.QueryMixed:    {MeanNDCGAt10: 0.3, MeanRecallAt10: 0.2},
			retrievalquality.QueryNegative: {NegativePassRate: 1},
		},
	}
	outcomes, err := outcomesFromTrainReport(report, retrievalquality.StrategyHybrid, params, identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 4 {
		t.Fatalf("outcomes=%d, want 4 positive kinds", len(outcomes))
	}
	if outcomes[0].Kind != retrievalquality.QueryExact || outcomes[0].NDCGAt10 != 0.9 ||
		outcomes[0].LatencyP95 != report.LatencyP95 {
		t.Fatalf("first outcome=%+v", outcomes[0])
	}
}

func TestCachedConfiguredStrategyAppliesQuotaAndCalibratedThreshold(t *testing.T) {
	cache := map[string]retrievalquality.SearchResult{
		"q": {Candidates: []retrievalquality.Candidate{
			{KnowledgeScope: "scope-a", ChunkID: "a", Score: 0.9},
			{KnowledgeScope: "scope-a", ChunkID: "b", Score: 0.8},
			{KnowledgeScope: "scope-a", ChunkID: "c", Score: 0.7},
		}},
	}
	strategy := cachedConfiguredStrategy{
		name: "cached", cache: cache,
		calibrator: retrievalquality.Calibrator{
			Breakpoints: []float64{1}, Values: []float64{0.2},
		},
		quota: 2, minimumRelevance: 0.25,
	}
	result, err := strategy.Search(context.Background(), retrievalquality.Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Abstained || len(result.Candidates) != 0 {
		t.Fatalf("result=%+v, want threshold abstention after quota", result)
	}
}

func TestPolicyGridIsTrainOnlyRetrievalGrid(t *testing.T) {
	if got := len(retrievalquality.DefaultPolicyGrid()); got != 81 {
		t.Fatalf("grid=%d, want 81 retrieval-only combinations", got)
	}
}

func TestPolicyArtifactPathIsRestrictedToIgnoredDirectory(t *testing.T) {
	for _, path := range []string{
		".cache/retrieval-quality/policy-v1.json",
		".cache/retrieval-quality/nested/policy.json",
	} {
		if _, err := validatePolicyArtifactPath(path); err != nil {
			t.Fatalf("valid path %q: %v", path, err)
		}
	}
	for _, path := range []string{
		"docs/teaching/policy.json",
		".cache/retrieval-quality/../../go.mod",
		"/tmp/policy.json",
		".cache/retrieval-quality/policy.txt",
	} {
		if _, err := validatePolicyArtifactPath(path); err == nil {
			t.Fatalf("unsafe path %q must fail", path)
		}
	}
}
