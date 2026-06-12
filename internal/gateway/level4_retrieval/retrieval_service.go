package retrieval

import world "offline-rag-go-lab/internal/gateway/level1_world"

// searcher 是 store 所需的最小接口，避免 retrieval 包直接依赖 level3_store 具体类型。
type searcher interface {
	Search(question string, topK int, threshold float64) []world.RetrievalHit
}

// RetrievalService 实现 hq.Retriever：标准化问题后委托 store 搜索。
type RetrievalService struct {
	store          searcher  // 知识库（通常是 MemoryKnowledgeStore）
	topK           int       // 最多返回条数
	scoreThreshold float64   // 分数下限
}

// NewRetrievalService 构造检索服务，topK 和 scoreThreshold 来自 Config。
func NewRetrievalService(store searcher, topK int, scoreThreshold float64) *RetrievalService {
	return &RetrievalService{
		store:          store,
		topK:           topK,
		scoreThreshold: scoreThreshold,
	}
}

// Retrieve 执行一次检索：记录原始问题、标准化问题、以及 store 返回的命中列表。
func (s *RetrievalService) Retrieve(question string) world.RetrievalResult {
	normalized := NormalizeText(question)
	return world.RetrievalResult{
		Question:           question,   // 保留原始问题供 debug 展示
		NormalizedQuestion: normalized, // 实际送入 Search 的字符串
		Hits:               s.store.Search(normalized, s.topK, s.scoreThreshold),
	}
}
