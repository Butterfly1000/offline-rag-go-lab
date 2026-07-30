package retrievalquality

import (
	"math"
	"sort"
	"time"
)

func recallAt(ranked []string, judgments map[string]int, k int) float64 {
	if len(judgments) == 0 || k <= 0 {
		return 0
	}
	found := 0
	seen := map[string]bool{}
	for index, id := range ranked {
		if index >= k {
			break
		}
		if judgments[id] > 0 && !seen[id] {
			found++
			seen[id] = true
		}
	}
	return float64(found) / float64(len(judgments))
}

func mrrAt(ranked []string, judgments map[string]int, k int) float64 {
	for index, id := range ranked {
		if index >= k {
			break
		}
		if judgments[id] > 0 {
			return 1 / float64(index+1)
		}
	}
	return 0
}

func ndcgAt(ranked []string, judgments map[string]int, k int) float64 {
	if len(judgments) == 0 || k <= 0 {
		return 0
	}
	var dcg float64
	for index, id := range ranked {
		if index >= k {
			break
		}
		relevance := judgments[id]
		if relevance > 0 {
			dcg += (math.Pow(2, float64(relevance)) - 1) / math.Log2(float64(index+2))
		}
	}
	grades := make([]int, 0, len(judgments))
	for _, relevance := range judgments {
		grades = append(grades, relevance)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	var ideal float64
	for index, relevance := range grades {
		if index >= k {
			break
		}
		ideal += (math.Pow(2, float64(relevance)) - 1) / math.Log2(float64(index+2))
	}
	if ideal == 0 {
		return 0
	}
	return dcg / ideal
}

func percentile(values []time.Duration, fraction float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	rank := int(math.Ceil(fraction * float64(len(ordered))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(ordered) {
		rank = len(ordered)
	}
	return ordered[rank-1]
}

func negativePassed(abstained bool, _ []Candidate) bool {
	return abstained
}
