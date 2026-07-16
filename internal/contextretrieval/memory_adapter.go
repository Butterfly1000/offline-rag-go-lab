package contextretrieval

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"offline-rag-go-lab/internal/memoryitem"
)

type MemoryVectorSearcher interface {
	Search(ctx context.Context, userID string, kind memoryitem.Kind, vector []float32, limit int) ([]memoryitem.SearchResult, error)
}

type MemoryQdrantSearcher struct {
	index MemoryVectorSearcher
}

func NewMemoryQdrantSearcher(index MemoryVectorSearcher) *MemoryQdrantSearcher {
	return &MemoryQdrantSearcher{index: index}
}

func (s *MemoryQdrantSearcher) Search(ctx context.Context, userID string, vector []float32, limit int) ([]Hit, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("memory search user_id is required")
	}
	if len(vector) == 0 {
		return nil, fmt.Errorf("memory search vector is required")
	}
	for index, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("memory search vector value %d must be finite", index)
		}
	}
	if limit <= 0 {
		return nil, fmt.Errorf("memory search limit must be positive: %d", limit)
	}
	if s == nil || s.index == nil {
		return nil, InfrastructureFailure(SourceMemory, fmt.Errorf("memory vector searcher is required"))
	}

	results, err := s.index.Search(ctx, userID, "", vector, limit)
	if err != nil {
		if memoryitem.IsQdrantDataError(err) {
			return nil, IntegrityFailure(SourceMemory, err)
		}
		return nil, InfrastructureFailure(SourceMemory, err)
	}
	hits := make([]Hit, 0, len(results))
	for index, result := range results {
		hit, err := memoryResultToHit(result, userID)
		if err != nil {
			return nil, IntegrityFailure(SourceMemory, fmt.Errorf("validate result %d: %w", index, err))
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func memoryResultToHit(result memoryitem.SearchResult, userID string) (Hit, error) {
	if result.ItemID <= 0 {
		return Hit{}, fmt.Errorf("memory item ID must be positive: %d", result.ItemID)
	}
	if strings.TrimSpace(result.UserID) != userID {
		return Hit{}, fmt.Errorf("memory result belongs to user %q, want %q", result.UserID, userID)
	}
	kind := strings.TrimSpace(string(result.Kind))
	key := strings.TrimSpace(result.Key)
	value := strings.TrimSpace(result.Value)
	if kind == "" || key == "" || value == "" || result.Version <= 0 {
		return Hit{}, fmt.Errorf("memory result kind, key, value and positive version are required")
	}
	hit, err := ValidateHit(Hit{
		Source:  SourceMemory,
		ID:      "memory:" + strconv.FormatInt(result.ItemID, 10),
		Content: fmt.Sprintf("%s/%s: %s", kind, key, value),
		Score:   result.Score,
		UserID:  userID,
		Kind:    kind,
		Metadata: map[string]string{
			"key": key, "version": strconv.FormatInt(result.Version, 10),
		},
	})
	if err != nil {
		return Hit{}, err
	}
	return hit, nil
}

var _ MemorySearcher = (*MemoryQdrantSearcher)(nil)
