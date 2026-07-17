package documentingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestEvaluationReportJSONUsesStableMetricNames(t *testing.T) {
	encoded, err := json.Marshal(EvaluationReport{MeanRecallAt3: 1, MeanMRRAt3: 0.5, Cases: []CaseResult{{RecallAt3: 1, MRRAt3: 0.5}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, field := range []string{`"mean_recall_at_3"`, `"mean_mrr_at_3"`, `"recall_at_3"`, `"mrr_at_3"`} {
		if !strings.Contains(text, field) {
			t.Fatalf("JSON %s lacks %s", text, field)
		}
	}
}

type fakeQueryEmbedder struct{ calls int }

func (e *fakeQueryEmbedder) Embed(_ context.Context, _ string, texts []string) ([][]float32, error) {
	e.calls++
	return [][]float32{{float32(queryKey(texts[0]))}}, nil
}
func queryKey(text string) int {
	total := 0
	for _, char := range text {
		total += int(char)
	}
	return total
}

type fakeScopedSearcher struct {
	hits   map[string][]EvaluationHit
	limits []int
}

func (s *fakeScopedSearcher) Search(_ context.Context, scope string, vector []float32, limit int) ([]EvaluationHit, error) {
	s.limits = append(s.limits, limit)
	return append([]EvaluationHit(nil), s.hits[fmt.Sprintf("%s/%d", scope, int(vector[0]))]...), nil
}

func validGoldenCases() []GoldenCase {
	cases := make([]GoldenCase, 10)
	for i := range cases {
		cases[i] = GoldenCase{CaseID: fmt.Sprintf("case-%02d", 10-i), Query: fmt.Sprintf("q%d", i), KnowledgeScope: "course", ExpectedChunkIDs: []string{fmt.Sprintf("expected-%d", i)}, ForbiddenChunkIDs: []string{"forbidden"}}
	}
	return cases
}

func TestEvaluateComputesRecallMRRAndStableOrder(t *testing.T) {
	cases := validGoldenCases()
	embedder := &fakeQueryEmbedder{}
	searcher := &fakeScopedSearcher{hits: map[string][]EvaluationHit{}}
	for i, item := range cases {
		searcher.hits[fmt.Sprintf("course/%d", queryKey(item.Query))] = []EvaluationHit{{ChunkID: "noise", KnowledgeScope: "course"}, {ChunkID: item.ExpectedChunkIDs[0], KnowledgeScope: "course"}}
		_ = i
	}
	report, err := Evaluate(context.Background(), cases, "bge-m3", embedder, searcher)
	if err != nil {
		t.Fatal(err)
	}
	if report.CaseCount != 10 || report.MeanRecallAt3 != 1 || report.MeanMRRAt3 != 0.5 || embedder.calls != 10 {
		t.Fatalf("report=%#v calls=%d", report, embedder.calls)
	}
	if report.Cases[0].CaseID != "case-01" || report.Cases[9].CaseID != "case-10" {
		t.Fatalf("order=%s...%s", report.Cases[0].CaseID, report.Cases[9].CaseID)
	}
	for _, limit := range searcher.limits {
		if limit != 3 {
			t.Fatalf("limit=%d", limit)
		}
	}
}

func TestEvaluateRejectsTooFewCasesDuplicateHitsAndCrossScope(t *testing.T) {
	cases := validGoldenCases()
	if _, err := Evaluate(context.Background(), cases[:9], "bge-m3", &fakeQueryEmbedder{}, &fakeScopedSearcher{}); err == nil {
		t.Fatal("fewer than ten cases must fail")
	}
	key := fmt.Sprintf("course/%d", queryKey(cases[0].Query))
	searcher := &fakeScopedSearcher{hits: map[string][]EvaluationHit{key: {{ChunkID: "same", KnowledgeScope: "course"}, {ChunkID: "same", KnowledgeScope: "course"}}}}
	if _, err := Evaluate(context.Background(), cases, "bge-m3", &fakeQueryEmbedder{}, searcher); err == nil {
		t.Fatal("duplicate hits must fail")
	}
	searcher.hits[key] = []EvaluationHit{{ChunkID: "x", KnowledgeScope: "other"}}
	if _, err := Evaluate(context.Background(), cases, "bge-m3", &fakeQueryEmbedder{}, searcher); err == nil {
		t.Fatal("cross-scope hit must fail")
	}
}

func TestEvaluateReportsForbiddenHitAndPartialRecall(t *testing.T) {
	cases := validGoldenCases()
	cases[0].ExpectedChunkIDs = []string{"a", "b"}
	key := fmt.Sprintf("course/%d", queryKey(cases[0].Query))
	searcher := &fakeScopedSearcher{hits: map[string][]EvaluationHit{key: {{ChunkID: "a", KnowledgeScope: "course"}, {ChunkID: "forbidden", KnowledgeScope: "course"}}}}
	report, err := Evaluate(context.Background(), cases, "bge-m3", &fakeQueryEmbedder{}, searcher)
	if err != nil {
		t.Fatal(err)
	}
	var result CaseResult
	for _, candidate := range report.Cases {
		if candidate.CaseID == cases[0].CaseID {
			result = candidate
		}
	}
	if result.RecallAt3 != 0.5 || result.MRRAt3 != 1 || len(result.ForbiddenHits) != 1 || report.Passed {
		t.Fatalf("result=%#v report=%#v", result, report)
	}
}
