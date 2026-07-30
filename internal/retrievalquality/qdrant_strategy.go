package retrievalquality

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type DenseQuerier interface {
	QueryDense(context.Context, string, []float32, int) ([]Candidate, error)
}

type SparseQuerier interface {
	QuerySparse(context.Context, string, SparseVector, int) ([]Candidate, error)
}

type QdrantDenseStrategy struct {
	query    DenseQuerier
	embedder Embedder
	model    string
}

func NewQdrantDenseStrategy(query DenseQuerier, embedder Embedder, model string) (*QdrantDenseStrategy, error) {
	model = strings.TrimSpace(model)
	if query == nil || embedder == nil || model == "" {
		return nil, fmt.Errorf("dense Qdrant query, embedder, and model are required")
	}
	return &QdrantDenseStrategy{query: query, embedder: embedder, model: model}, nil
}

func (s *QdrantDenseStrategy) Name() string { return "qdrant_dense" }

func (s *QdrantDenseStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil || s.query == nil || s.embedder == nil {
		return SearchResult{}, fmt.Errorf("dense Qdrant strategy is not initialized")
	}
	if strings.TrimSpace(query.Text) == "" || strings.TrimSpace(query.KnowledgeScope) == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("dense query text, scope, and positive limit are required")
	}
	started := time.Now()
	vectors, err := s.embedder.Embed(ctx, s.model, []string{query.Text})
	if err != nil {
		return SearchResult{}, &InfrastructureError{Err: fmt.Errorf("embed dense query: %w", err)}
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return SearchResult{}, &IntegrityError{Err: fmt.Errorf("dense query embedding count or dimension is invalid")}
	}
	candidates, err := s.query.QueryDense(ctx, query.KnowledgeScope, vectors[0], query.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Candidates: candidates, Duration: time.Since(started)}, nil
}

type QdrantSparseStrategy struct {
	query   SparseQuerier
	encoder *SparseEncoder
}

func NewQdrantSparseStrategy(query SparseQuerier, encoder *SparseEncoder) (*QdrantSparseStrategy, error) {
	if query == nil || encoder == nil {
		return nil, fmt.Errorf("sparse Qdrant query and encoder are required")
	}
	return &QdrantSparseStrategy{query: query, encoder: encoder}, nil
}

func (s *QdrantSparseStrategy) Name() string { return "qdrant_sparse_field_bm25" }

func (s *QdrantSparseStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil || s.query == nil || s.encoder == nil {
		return SearchResult{}, fmt.Errorf("sparse Qdrant strategy is not initialized")
	}
	if strings.TrimSpace(query.KnowledgeScope) == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("sparse query scope and positive limit are required")
	}
	started := time.Now()
	vector, err := s.encoder.EncodeQuery(query.Text)
	if err != nil {
		return SearchResult{}, err
	}
	candidates, err := s.query.QuerySparse(ctx, query.KnowledgeScope, vector, query.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Candidates: candidates, Duration: time.Since(started)}, nil
}
