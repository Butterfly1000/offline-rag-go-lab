package retrievalquality

import (
	"fmt"
	"math"
)

type RegressionResult struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures"`
}

func CheckRegression(baseline, candidate Report, maxLatencyRatio float64) RegressionResult {
	result := RegressionResult{Passed: true, Failures: []string{}}
	if maxLatencyRatio <= 0 || math.IsNaN(maxLatencyRatio) || math.IsInf(maxLatencyRatio, 0) {
		result.Failures = append(result.Failures, "latency ratio must be finite and positive")
	}
	if candidate.ScopeIsolation != 1 {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"scope isolation=%g, want 1", candidate.ScopeIsolation,
		))
	}
	if candidate.ForbiddenHitCount != 0 {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"forbidden hit count=%d, want 0", candidate.ForbiddenHitCount,
		))
	}
	if candidate.NegativePassRate < baseline.NegativePassRate {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"negative pass rate=%g below baseline %g",
			candidate.NegativePassRate, baseline.NegativePassRate,
		))
	}
	if candidate.MeanNDCGAt10 < baseline.MeanNDCGAt10 {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"NDCG@10=%g below baseline %g", candidate.MeanNDCGAt10, baseline.MeanNDCGAt10,
		))
	}
	if candidate.MeanRecallAt10 < baseline.MeanRecallAt10 {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"Recall@10=%g below baseline %g", candidate.MeanRecallAt10, baseline.MeanRecallAt10,
		))
	}
	if maxLatencyRatio > 0 && finite(maxLatencyRatio) {
		maxLatency := float64(baseline.LatencyP95) * maxLatencyRatio
		if float64(candidate.LatencyP95) > maxLatency {
			result.Failures = append(result.Failures, fmt.Sprintf(
				"latency p95=%s exceeds %.2fx baseline %s",
				candidate.LatencyP95, maxLatencyRatio, baseline.LatencyP95,
			))
		}
	}
	result.Passed = len(result.Failures) == 0
	return result
}
