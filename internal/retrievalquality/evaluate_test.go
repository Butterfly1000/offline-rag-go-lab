package retrievalquality

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fixedStrategy struct {
	name    string
	results map[string]SearchResult
	errs    map[string]error
}

func (s fixedStrategy) Name() string { return s.name }

func (s fixedStrategy) Search(_ context.Context, query Query) (SearchResult, error) {
	if err := s.errs[query.CaseID]; err != nil {
		return SearchResult{}, err
	}
	return s.results[query.CaseID], nil
}

func TestEvaluateReportsQualitySafetyLatencyAndKinds(t *testing.T) {
	dataset := evaluationDataset()
	strategy := fixedStrategy{name: "fixed", results: map[string]SearchResult{
		"case-a": {
			Candidates: []Candidate{
				{ChunkID: "a", KnowledgeScope: "scope-a", Score: 0.9},
				{ChunkID: "b", KnowledgeScope: "scope-a", Score: 0.8},
			},
			Duration: 2 * time.Millisecond,
		},
		"case-b": {Abstained: true, Duration: 8 * time.Millisecond},
	}}
	report, err := Evaluate(context.Background(), dataset, SplitValidation, strategy, 10)
	if err != nil {
		t.Fatal(err)
	}
	if report.CaseCount != 2 || report.MeanRecallAt10 != 1 || report.MeanMRRAt10 != 1 {
		t.Fatalf("unexpected ranking report: %+v", report)
	}
	if report.NegativePassRate != 1 || report.ScopeIsolation != 1 || report.ForbiddenHitCount != 0 {
		t.Fatalf("unexpected safety report: %+v", report)
	}
	if report.LatencyP50 != 2*time.Millisecond || report.LatencyP95 != 8*time.Millisecond {
		t.Fatalf("latencies p50=%s p95=%s", report.LatencyP50, report.LatencyP95)
	}
	if len(report.ByKind) != 2 || report.Cases[0].CaseID != "case-a" {
		t.Fatalf("report is not grouped/stable: %+v", report)
	}
}

func TestEvaluateRejectsCrossScopeCandidate(t *testing.T) {
	dataset := evaluationDataset()
	strategy := fixedStrategy{name: "fixed", results: map[string]SearchResult{
		"case-a": {Candidates: []Candidate{{ChunkID: "other", KnowledgeScope: "scope-b"}}},
		"case-b": {Abstained: true},
	}}
	_, err := Evaluate(context.Background(), dataset, SplitValidation, strategy, 10)
	if err == nil || !strings.Contains(err.Error(), "cross-scope") {
		t.Fatalf("error=%v, want cross-scope hard failure", err)
	}
}

func TestEvaluateRejectsCandidateOutsideDatasetCorpus(t *testing.T) {
	dataset := evaluationDataset()
	strategy := fixedStrategy{name: "fixed", results: map[string]SearchResult{
		"case-a": {Candidates: []Candidate{{ChunkID: "invented", KnowledgeScope: "scope-a"}}},
		"case-b": {Abstained: true},
	}}
	_, err := Evaluate(context.Background(), dataset, SplitValidation, strategy, 10)
	if err == nil || !strings.Contains(err.Error(), "unknown corpus chunk") {
		t.Fatalf("error=%v, want unknown corpus chunk hard failure", err)
	}
}

func TestEvaluateClassifiesStrategyFailure(t *testing.T) {
	dataset := evaluationDataset()
	strategy := fixedStrategy{name: "dense", results: map[string]SearchResult{}, errs: map[string]error{
		"case-a": errors.New("embedding unavailable"),
	}}
	report, err := Evaluate(context.Background(), dataset, SplitValidation, strategy, 10)
	if err == nil {
		t.Fatal("expected strategy failure")
	}
	if report.FailureStage != FailureDenseRecall {
		t.Fatalf("failure_stage=%q, want %q", report.FailureStage, FailureDenseRecall)
	}
}

func TestStageForStrategyClassifiesRerankerBeforeHybridBase(t *testing.T) {
	if got := stageForStrategy("hybrid_rerank_diversity"); got != FailureRerank {
		t.Fatalf("failure_stage=%q, want %q", got, FailureRerank)
	}
}

func evaluationDataset() Dataset {
	return Dataset{
		Manifest: DatasetManifest{DatasetID: "test", Version: "v1", TrainCount: 0, ValidationCount: 2},
		Corpus: []Chunk{
			{ChunkID: "a", KnowledgeScope: "scope-a"},
			{ChunkID: "b", KnowledgeScope: "scope-a"},
			{ChunkID: "other", KnowledgeScope: "scope-b"},
		},
		Cases: []GoldenCase{
			{
				CaseID: "case-b", Query: "unsupported", KnowledgeScope: "scope-a",
				Kind: QueryNegative, Split: SplitValidation, ForbiddenChunkIDs: []string{"other"},
			},
			{
				CaseID: "case-a", Query: "find a", KnowledgeScope: "scope-a",
				Kind: QueryExact, Split: SplitValidation,
				Judgments:         []Judgment{{ChunkID: "a", Relevance: 3}},
				ForbiddenChunkIDs: []string{"other"},
			},
		},
	}
}
