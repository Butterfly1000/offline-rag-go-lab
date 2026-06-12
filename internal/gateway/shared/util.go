package shared

import (
	"math"
	"os"
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
)

func MustMkdirAll(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		panic(err)
	}
}

func Truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func Round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func ValueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func ChunkIDs(hits []world.RetrievalHit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		out = append(out, hit.ChunkID)
	}
	return out
}

func LimitHits(hits []world.RetrievalHit, max int) []world.RetrievalHit {
	if max <= 0 || len(hits) <= max {
		return hits
	}
	return hits[:max]
}

func DedupeHitsByContent(hits []world.RetrievalHit) []world.RetrievalHit {
	if len(hits) <= 1 {
		return hits
	}

	seen := make(map[string]struct{}, len(hits))
	out := make([]world.RetrievalHit, 0, len(hits))
	for _, hit := range hits {
		key := retrieval.NormalizeText(hit.Title + " " + hit.Text)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, hit)
	}
	return out
}
