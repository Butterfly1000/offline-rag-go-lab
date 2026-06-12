package retrieval

import world "offline-rag-go-lab/internal/gateway/level1_world"

type searcher interface {
	Search(question string, topK int, threshold float64) []world.RetrievalHit
}

type RetrievalService struct {
	store          searcher
	topK           int
	scoreThreshold float64
}

func NewRetrievalService(store searcher, topK int, scoreThreshold float64) *RetrievalService {
	return &RetrievalService{
		store:          store,
		topK:           topK,
		scoreThreshold: scoreThreshold,
	}
}

func (s *RetrievalService) Retrieve(question string) world.RetrievalResult {
	normalized := NormalizeText(question)
	return world.RetrievalResult{
		Question:           question,
		NormalizedQuestion: normalized,
		Hits:               s.store.Search(normalized, s.topK, s.scoreThreshold),
	}
}
