package retrievalquality

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRerankedStrategyReranksDiversifiesAndTracksStats(t *testing.T) {
	base := &recordingStrategy{name: "hybrid", result: SearchResult{
		Candidates: []Candidate{
			{KnowledgeScope: "scope-a", DocumentID: "d1", HeadingPath: "h1", ChunkID: "a"},
			{KnowledgeScope: "scope-a", DocumentID: "d1", HeadingPath: "h1", ChunkID: "b"},
			{KnowledgeScope: "scope-a", DocumentID: "d1", HeadingPath: "h1", ChunkID: "c"},
			{KnowledgeScope: "scope-a", DocumentID: "d2", HeadingPath: "h2", ChunkID: "d"},
		},
	}}
	reranker := fixedReranker{scores: []RerankScore{
		{CandidateID: "a", Relevance: 0.7},
		{CandidateID: "b", Relevance: 0.9},
		{CandidateID: "c", Relevance: 0.8},
		{CandidateID: "d", Relevance: 0.6},
	}}
	strategy, err := NewRerankedStrategy(base, reranker, DiversityLimits{PerDocument: 3, PerHeading: 2})
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.ChunkID)
	}
	if want := []string{"b", "c", "d"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v, want %v", ids, want)
	}
	if base.recordedQuery().Limit != 20 {
		t.Fatalf("base limit=%d, want 20", base.recordedQuery().Limit)
	}
	stats := strategy.Stats()
	if stats.Calls != 1 || stats.Used != 1 || stats.Fallbacks != 0 ||
		stats.DiversitySkipped != 1 || stats.LatencyP50 < 0 || stats.LatencyP95 < 0 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestRerankedStrategyFallsBackWithoutChangingBaseOrder(t *testing.T) {
	base := &recordingStrategy{name: "hybrid", result: SearchResult{Candidates: rerankCandidates()}}
	strategy, err := NewRerankedStrategy(
		base,
		fixedReranker{err: errors.New("ollama unavailable")},
		DiversityLimits{PerDocument: 3, PerHeading: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.ChunkID)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("fallback ids=%v, want %v", ids, want)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "ollama unavailable") {
		t.Fatalf("warnings=%v", result.Warnings)
	}
	stats := strategy.Stats()
	if stats.Calls != 1 || stats.Used != 0 || stats.Fallbacks != 1 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestRerankedStrategyTreatsCandidateOwnershipFailuresAsHard(t *testing.T) {
	for _, candidates := range [][]Candidate{
		{
			{KnowledgeScope: "scope-a", DocumentID: "d1", HeadingPath: "h1", ChunkID: "a"},
			{KnowledgeScope: "scope-b", DocumentID: "d2", HeadingPath: "h2", ChunkID: "b"},
		},
		{
			{KnowledgeScope: "scope-a", DocumentID: "", HeadingPath: "h1", ChunkID: "a"},
		},
	} {
		base := &recordingStrategy{name: "hybrid", result: SearchResult{Candidates: candidates}}
		strategy, err := NewRerankedStrategy(
			base,
			fixedReranker{scores: []RerankScore{{CandidateID: "a", Relevance: 1}}},
			DiversityLimits{PerDocument: 3, PerHeading: 2},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := strategy.Search(context.Background(), Query{
			CaseID: "q", Text: "query", KnowledgeScope: "scope-a", Limit: 3,
		}); err == nil {
			t.Fatalf("candidates=%v should hard fail", candidates)
		}
	}
}
