// Package compression 在检索命中进入 prompt/生成器之前做体量控制。
package compression

import (
	world "offline-rag-go-lab/internal/gateway/level1_world"
	"offline-rag-go-lab/internal/gateway/shared"
)

// SimpleCompressor 默认压缩器：去重 → 限量 → 单条截断（无 LLM 摘要）。
type SimpleCompressor struct{}

// Compress 对 hits 依次去重、限制条数、截断每条 Text，返回新 slice（不修改原 hit 的 Text）。
func (c SimpleCompressor) Compress(hits []world.RetrievalHit, maxChunks int, maxChars int) []world.RetrievalHit {
	deduped := shared.DedupeHitsByContent(hits) // 相同标题+正文只留第一条
	limited := shared.LimitHits(deduped, maxChunks)

	out := make([]world.RetrievalHit, 0, len(limited))
	for _, hit := range limited {
		cloned := hit                        // 结构体值拷贝
		cloned.Text = shared.Truncate(hit.Text, maxChars) // 只截断副本上的 Text
		out = append(out, cloned)
	}
	return out
}
