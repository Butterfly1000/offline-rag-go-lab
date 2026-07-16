package contextretrieval

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type QueryEmbedder interface {
	Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}

type MemorySearcher interface {
	Search(ctx context.Context, userID string, vector []float32, limit int) ([]Hit, error)
}

type DocumentSearcher interface {
	Search(ctx context.Context, knowledgeScope string, vector []float32, limit int) ([]Hit, error)
}

type DualRequest struct {
	Query          string
	UserID         string
	KnowledgeScope string
	UseMemory      bool
	UseDocuments   bool
	MemoryLimit    int
	DocumentLimit  int
}

type DualResult struct {
	MemoryHits   []Hit
	DocumentHits []Hit
	Warnings     []string
}

type DualRetriever struct {
	embedder       QueryEmbedder
	embeddingModel string
	memory         MemorySearcher
	documents      DocumentSearcher
}

func NewDualRetriever(embedder QueryEmbedder, embeddingModel string, memory MemorySearcher, documents DocumentSearcher) *DualRetriever {
	return &DualRetriever{
		embedder: embedder, embeddingModel: strings.TrimSpace(embeddingModel),
		memory: memory, documents: documents,
	}
}

func (r *DualRetriever) Retrieve(ctx context.Context, req DualRequest) (DualResult, error) {
	req, err := validateDualRequest(r, req)
	if err != nil {
		return DualResult{}, err
	}
	vectors, err := r.embedder.Embed(ctx, r.embeddingModel, []string{req.Query})
	if err != nil {
		return DualResult{Warnings: []string{"query embedding unavailable: " + err.Error()}}, nil
	}
	if err := validateQueryEmbedding(vectors); err != nil {
		return DualResult{Warnings: []string{"query embedding unavailable: " + err.Error()}}, nil
	}
	vector := vectors[0]

	type sourceResult struct {
		source Source
		hits   []Hit
		err    error
	}
	count := 0
	if req.UseMemory {
		count++
	}
	if req.UseDocuments {
		count++
	}
	results := make(chan sourceResult, count)
	if req.UseMemory {
		go func() {
			hits, searchErr := r.memory.Search(ctx, req.UserID, vector, req.MemoryLimit)
			results <- sourceResult{source: SourceMemory, hits: hits, err: searchErr}
		}()
	}
	if req.UseDocuments {
		go func() {
			hits, searchErr := r.documents.Search(ctx, req.KnowledgeScope, vector, req.DocumentLimit)
			results <- sourceResult{source: SourceDocument, hits: hits, err: searchErr}
		}()
	}

	bySource := make(map[Source]sourceResult, count)
	for range count {
		item := <-results
		bySource[item.source] = item
	}
	result := DualResult{}
	for _, source := range []Source{SourceMemory, SourceDocument} {
		item, started := bySource[source]
		if !started {
			continue
		}
		if item.err != nil {
			if IsInfrastructureFailure(item.err) {
				result.Warnings = append(result.Warnings, item.err.Error())
				continue
			}
			return DualResult{}, item.err
		}
		hits, err := validateRetrievedHits(source, item.hits, req)
		if err != nil {
			return DualResult{}, err
		}
		if source == SourceMemory {
			result.MemoryHits = hits
		} else {
			result.DocumentHits = hits
		}
	}
	return result, nil
}

func validateDualRequest(r *DualRetriever, req DualRequest) (DualRequest, error) {
	if r == nil || r.embedder == nil || r.embeddingModel == "" {
		return DualRequest{}, fmt.Errorf("dual retriever embedder and embedding model are required")
	}
	req.Query = strings.TrimSpace(req.Query)
	req.UserID = strings.TrimSpace(req.UserID)
	req.KnowledgeScope = strings.TrimSpace(req.KnowledgeScope)
	if req.Query == "" {
		return DualRequest{}, fmt.Errorf("dual retrieval query is required")
	}
	if !req.UseMemory && !req.UseDocuments {
		return DualRequest{}, fmt.Errorf("at least one retrieval source must be enabled")
	}
	if req.UseMemory && (r.memory == nil || req.UserID == "" || req.MemoryLimit <= 0) {
		return DualRequest{}, fmt.Errorf("enabled memory retrieval requires a searcher, user_id and positive limit")
	}
	if req.UseDocuments && (r.documents == nil || req.KnowledgeScope == "" || req.DocumentLimit <= 0) {
		return DualRequest{}, fmt.Errorf("enabled document retrieval requires a searcher, knowledge_scope and positive limit")
	}
	return req, nil
}

func validateQueryEmbedding(vectors [][]float32) error {
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return fmt.Errorf("embedding response must contain one non-empty vector")
	}
	for index, value := range vectors[0] {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding value %d must be finite", index)
		}
	}
	return nil
}

func validateRetrievedHits(source Source, hits []Hit, req DualRequest) ([]Hit, error) {
	validated := make([]Hit, 0, len(hits))
	for index, hit := range hits {
		hit, err := ValidateHit(hit)
		if err != nil {
			return nil, IntegrityFailure(source, fmt.Errorf("validate hit %d: %w", index, err))
		}
		if hit.Source != source {
			return nil, IntegrityFailure(source, fmt.Errorf("hit %d source is %q", index, hit.Source))
		}
		if source == SourceMemory && hit.UserID != req.UserID {
			return nil, IntegrityFailure(source, fmt.Errorf("hit %d belongs to user %q, want %q", index, hit.UserID, req.UserID))
		}
		if source == SourceDocument && hit.KnowledgeScope != req.KnowledgeScope {
			return nil, IntegrityFailure(source, fmt.Errorf("hit %d belongs to knowledge_scope %q, want %q", index, hit.KnowledgeScope, req.KnowledgeScope))
		}
		validated = append(validated, hit)
	}
	return validated, nil
}
