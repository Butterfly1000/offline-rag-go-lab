package compression

import (
	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

type SimpleCompressor struct{}

func (c SimpleCompressor) Compress(hits []world.RetrievalHit, maxChunks int, maxChars int) []world.RetrievalHit {
	deduped := shared.DedupeHitsByContent(hits)
	limited := shared.LimitHits(deduped, maxChunks)

	out := make([]world.RetrievalHit, 0, len(limited))
	for _, hit := range limited {
		cloned := hit
		cloned.Text = shared.Truncate(hit.Text, maxChars)
		out = append(out, cloned)
	}
	return out
}
