package retrievalquality

import (
	"math"
	"testing"
	"time"
)

func TestRankingMetricsUseHandCheckedGradedJudgments(t *testing.T) {
	judgments := map[string]int{"a": 3, "b": 2, "c": 1}
	ranked := []string{"x", "b", "a", "c"}

	if got := recallAt(ranked, judgments, 3); math.Abs(got-2.0/3.0) > 1e-9 {
		t.Fatalf("Recall@3=%v, want %v", got, 2.0/3.0)
	}
	if got := mrrAt(ranked, judgments, 10); got != 0.5 {
		t.Fatalf("MRR@10=%v, want 0.5", got)
	}
	// DCG = 3/log2(3) + 7/log2(4) + 1/log2(5).
	// IDCG = 7 + 3/log2(3) + 1/log2(4).
	wantNDCG := (3/math.Log2(3) + 7/math.Log2(4) + 1/math.Log2(5)) /
		(7 + 3/math.Log2(3) + 1/math.Log2(4))
	if got := ndcgAt(ranked, judgments, 10); math.Abs(got-wantNDCG) > 1e-9 {
		t.Fatalf("NDCG@10=%v, want %v", got, wantNDCG)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []time.Duration{5 * time.Millisecond, time.Millisecond, 4 * time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}
	if got := percentile(values, 0.50); got != 3*time.Millisecond {
		t.Fatalf("p50=%s, want 3ms", got)
	}
	if got := percentile(values, 0.95); got != 5*time.Millisecond {
		t.Fatalf("p95=%s, want 5ms", got)
	}
}

func TestNegativeCasePassesOnlyWhenStrategyAbstains(t *testing.T) {
	if !negativePassed(true, nil) {
		t.Fatal("abstained negative case should pass")
	}
	if negativePassed(false, nil) {
		t.Fatal("non-abstained negative case should fail even with no candidates")
	}
}
