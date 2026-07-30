package retrievalquality

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Embedder interface {
	Embed(context.Context, string, []string) ([][]float32, error)
}

type denseMemoryItem struct {
	chunk  Chunk
	vector []float32
}

type DenseMemoryStrategy struct {
	embedder Embedder
	model    string
	items    []denseMemoryItem
	dim      int
}

func NewDenseMemoryStrategy(ctx context.Context, embedder Embedder, model string, corpus []Chunk) (*DenseMemoryStrategy, error) {
	if embedder == nil {
		return nil, fmt.Errorf("dense memory embedder is required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("dense memory model is required")
	}
	if len(corpus) == 0 {
		return nil, fmt.Errorf("dense memory corpus is required")
	}
	texts := make([]string, len(corpus))
	for i, chunk := range corpus {
		if strings.TrimSpace(chunk.ChunkID) == "" || strings.TrimSpace(chunk.KnowledgeScope) == "" || strings.TrimSpace(chunk.Content) == "" {
			return nil, fmt.Errorf("dense corpus chunk %d requires chunk_id, knowledge_scope, and content", i)
		}
		texts[i] = chunk.Content
	}
	vectors, err := embedder.Embed(ctx, model, texts)
	if err != nil {
		return nil, fmt.Errorf("embed dense corpus: %w", err)
	}
	if len(vectors) != len(corpus) {
		return nil, fmt.Errorf("dense corpus embedding count=%d, want %d", len(vectors), len(corpus))
	}
	dim := len(vectors[0])
	if dim == 0 {
		return nil, fmt.Errorf("dense corpus vector dimension must be positive")
	}
	items := make([]denseMemoryItem, len(corpus))
	for i, vector := range vectors {
		if err := validateDenseVector(vector, dim); err != nil {
			return nil, fmt.Errorf("dense corpus chunk %q: %w", corpus[i].ChunkID, err)
		}
		items[i] = denseMemoryItem{chunk: corpus[i], vector: append([]float32(nil), vector...)}
	}
	return &DenseMemoryStrategy{embedder: embedder, model: model, items: items, dim: dim}, nil
}

func (s *DenseMemoryStrategy) Name() string { return "dense_memory_bge_m3" }

func (s *DenseMemoryStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil || s.embedder == nil || s.dim <= 0 {
		return SearchResult{}, fmt.Errorf("dense memory strategy is not initialized")
	}
	query.Text = strings.TrimSpace(query.Text)
	query.KnowledgeScope = strings.TrimSpace(query.KnowledgeScope)
	if query.Text == "" || query.KnowledgeScope == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("dense query text, knowledge_scope, and positive limit are required")
	}
	started := time.Now()
	vectors, err := s.embedder.Embed(ctx, s.model, []string{query.Text})
	if err != nil {
		return SearchResult{}, fmt.Errorf("embed dense query: %w", err)
	}
	if len(vectors) != 1 {
		return SearchResult{}, fmt.Errorf("dense query embedding count=%d, want 1", len(vectors))
	}
	if err := validateDenseVector(vectors[0], s.dim); err != nil {
		return SearchResult{}, fmt.Errorf("dense query embedding: %w", err)
	}
	candidates := make([]Candidate, 0, len(s.items))
	for _, item := range s.items {
		if item.chunk.KnowledgeScope != query.KnowledgeScope {
			continue
		}
		candidates = append(candidates, Candidate{
			KnowledgeScope: item.chunk.KnowledgeScope,
			DocumentID:     item.chunk.DocumentID,
			ChunkID:        item.chunk.ChunkID,
			HeadingPath:    item.chunk.HeadingPath,
			Content:        item.chunk.Content,
			Score:          cosine(vectors[0], item.vector),
			Reason:         "dense_cosine",
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ChunkID < candidates[j].ChunkID
	})
	if len(candidates) > query.Limit {
		candidates = candidates[:query.Limit]
	}
	return SearchResult{Candidates: candidates, Duration: time.Since(started)}, nil
}

func validateDenseVector(vector []float32, dim int) error {
	if len(vector) != dim {
		return fmt.Errorf("vector dimension=%d, want %d", len(vector), dim)
	}
	for index, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("vector value %d must be finite", index)
		}
	}
	return nil
}

func cosine(left, right []float32) float64 {
	var dot, leftNorm, rightNorm float64
	for i := range left {
		l, r := float64(left[i]), float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
