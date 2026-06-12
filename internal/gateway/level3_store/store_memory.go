package store

import (
	"sort"

	world "offline-rag-go-lab/internal/gateway/level1_world"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
)

type MemoryKnowledgeStore struct {
	chunks []world.KnowledgeChunk
}

func NewMemoryKnowledgeStore() *MemoryKnowledgeStore {
	return &MemoryKnowledgeStore{}
}

func (s *MemoryKnowledgeStore) Upsert(chunks []world.KnowledgeChunk) {
	existing := make(map[string]world.KnowledgeChunk, len(s.chunks))
	for _, chunk := range s.chunks {
		existing[chunk.ChunkID] = chunk
	}
	for _, chunk := range chunks {
		existing[chunk.ChunkID] = chunk
	}

	s.chunks = s.chunks[:0]
	for _, chunk := range existing {
		s.chunks = append(s.chunks, chunk)
	}

	sort.Slice(s.chunks, func(i, j int) bool {
		if s.chunks[i].DocumentID == s.chunks[j].DocumentID {
			return s.chunks[i].ChunkIndex < s.chunks[j].ChunkIndex
		}
		return s.chunks[i].DocumentID < s.chunks[j].DocumentID
	})
}

func (s *MemoryKnowledgeStore) Search(question string, topK int, threshold float64) []world.RetrievalHit {
	hits := make([]world.RetrievalHit, 0)
	for _, chunk := range s.chunks {
		score := retrieval.Similarity(question, chunk.Title+" "+chunk.Text)
		if score < threshold {
			continue
		}
		hits = append(hits, world.RetrievalHit{
			KnowledgeChunk: chunk,
			Score:          score,
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ChunkID < hits[j].ChunkID
		}
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}
