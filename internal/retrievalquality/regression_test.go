package retrievalquality

import (
	"strings"
	"testing"
	"time"
)

func TestCheckRegressionAcceptsEqualOrBetterCandidate(t *testing.T) {
	baseline := regressionReport()
	candidate := baseline
	candidate.MeanNDCGAt10 = 0.91
	result := CheckRegression(baseline, candidate, 3)
	if !result.Passed || len(result.Failures) != 0 {
		t.Fatalf("result=%+v", result)
	}
}

func TestCheckRegressionReportsEveryGate(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Report)
		want string
	}{
		{name: "scope", edit: func(r *Report) { r.ScopeIsolation = 0.99 }, want: "scope"},
		{name: "forbidden", edit: func(r *Report) { r.ForbiddenHitCount = 1 }, want: "forbidden"},
		{name: "negative", edit: func(r *Report) { r.NegativePassRate = 0.4 }, want: "negative"},
		{name: "ndcg", edit: func(r *Report) { r.MeanNDCGAt10 = 0.89 }, want: "NDCG"},
		{name: "recall", edit: func(r *Report) { r.MeanRecallAt10 = 0.89 }, want: "Recall"},
		{name: "latency", edit: func(r *Report) { r.LatencyP95 = 301 * time.Millisecond }, want: "latency"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := regressionReport()
			candidate := baseline
			tt.edit(&candidate)
			result := CheckRegression(baseline, candidate, 3)
			if result.Passed || len(result.Failures) != 1 ||
				!strings.Contains(result.Failures[0], tt.want) {
				t.Fatalf("result=%+v, want %q failure", result, tt.want)
			}
		})
	}
}

func regressionReport() Report {
	return Report{
		MeanRecallAt10:    0.9,
		MeanNDCGAt10:      0.9,
		NegativePassRate:  0.5,
		ScopeIsolation:    1,
		ForbiddenHitCount: 0,
		LatencyP95:        100 * time.Millisecond,
	}
}
