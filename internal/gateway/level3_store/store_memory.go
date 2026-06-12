// Package store 提供知识库默认实现：内存 slice，模拟向量库/Qdrant 的 Upsert 与 Search。
package store

import (
	"sort" // Upsert 后按 DocumentID、ChunkIndex 排序；Search 后按 Score 降序

	world "offline-rag-go-lab/internal/gateway/level1_world"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
)

// MemoryKnowledgeStore 进程内内存知识库，重启后数据丢失。
type MemoryKnowledgeStore struct {
	chunks []world.KnowledgeChunk // 所有 chunk 存在一个 slice 里
}

// NewMemoryKnowledgeStore 创建空内存 store。
func NewMemoryKnowledgeStore() *MemoryKnowledgeStore {
	return &MemoryKnowledgeStore{}
}

// Upsert 插入或更新 chunk：相同 ChunkID 会覆盖，最后按文档 ID 和 chunk 序号排序。
func (s *MemoryKnowledgeStore) Upsert(chunks []world.KnowledgeChunk) {
	// 先把已有 chunk 放进 map，key 为 ChunkID
	existing := make(map[string]world.KnowledgeChunk, len(s.chunks))
	for _, chunk := range s.chunks {
		existing[chunk.ChunkID] = chunk
	}
	// 新 chunk 写入 map，同 ID 覆盖
	for _, chunk := range chunks {
		existing[chunk.ChunkID] = chunk
	}

	// 清空 slice 再重建（map 遍历顺序随机，所以后面要 sort）
	s.chunks = s.chunks[:0]
	for _, chunk := range existing {
		s.chunks = append(s.chunks, chunk)
	}

	// 稳定排序：先按 DocumentID，再按 ChunkIndex
	sort.Slice(s.chunks, func(i, j int) bool {
		if s.chunks[i].DocumentID == s.chunks[j].DocumentID {
			return s.chunks[i].ChunkIndex < s.chunks[j].ChunkIndex
		}
		return s.chunks[i].DocumentID < s.chunks[j].DocumentID
	})
}

// Search 遍历所有 chunk 算相似度，过滤低于 threshold 的，按分数降序取 topK。
// 当前用教学版 token 重叠相似度（非真实 embedding 向量）。
func (s *MemoryKnowledgeStore) Search(question string, topK int, threshold float64) []world.RetrievalHit {
	hits := make([]world.RetrievalHit, 0)
	for _, chunk := range s.chunks {
		// 用「标题+正文」与 question 算分，标题词也会参与匹配
		score := retrieval.Similarity(question, chunk.Title+" "+chunk.Text)
		if score < threshold {
			continue // 低于阈值视为不相关
		}
		hits = append(hits, world.RetrievalHit{
			KnowledgeChunk: chunk,
			Score:          score,
		})
	}

	// 分数高的排前面；同分按 ChunkID 字典序保证稳定
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ChunkID < hits[j].ChunkID
		}
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > topK {
		hits = hits[:topK] // 只保留前 topK 条
	}
	return hits
}
