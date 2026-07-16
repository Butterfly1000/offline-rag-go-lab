package memoryitem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearchResult struct {
	ItemID  int64
	Score   float64
	UserID  string
	Kind    Kind
	Key     string
	Value   string
	Version int64
}

type QdrantIndexer struct {
	baseURL    string
	collection string
	client     *http.Client
}

func NewQdrantIndexer(baseURL, collection string) *QdrantIndexer {
	return &QdrantIndexer{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		collection: strings.TrimSpace(collection),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (q *QdrantIndexer) EnsureCollection(ctx context.Context, vectorSize int) error {
	collectionPath, err := q.collectionPath()
	if err != nil {
		return err
	}
	if vectorSize <= 0 {
		return fmt.Errorf("Qdrant vector size must be positive: %d", vectorSize)
	}

	var existing qdrantCollectionResponse
	status, err := q.doJSON(ctx, http.MethodGet, collectionPath, nil, &existing, true)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		request := struct {
			Vectors qdrantVectorConfig `json:"vectors"`
		}{Vectors: qdrantVectorConfig{Size: vectorSize, Distance: "Cosine"}}
		if _, err := q.doJSON(ctx, http.MethodPut, collectionPath, request, nil, false); err != nil {
			return fmt.Errorf("create Qdrant collection: %w", err)
		}
	} else {
		vectors := existing.Result.Config.Params.Vectors
		if vectors.Size != vectorSize || !strings.EqualFold(vectors.Distance, "Cosine") {
			return fmt.Errorf(
				"Qdrant collection %q uses size=%d distance=%s, want size=%d distance=Cosine",
				q.collection, vectors.Size, vectors.Distance, vectorSize,
			)
		}
	}

	for _, field := range []string{"user_id", "kind"} {
		body := struct {
			FieldName   string `json:"field_name"`
			FieldSchema string `json:"field_schema"`
		}{FieldName: field, FieldSchema: "keyword"}
		if _, err := q.doJSON(ctx, http.MethodPut, collectionPath+"/index?wait=true", body, nil, false); err != nil {
			return fmt.Errorf("create Qdrant payload index %q: %w", field, err)
		}
	}
	return nil
}

func (q *QdrantIndexer) Upsert(ctx context.Context, item Item, vector []float32, model string) error {
	collectionPath, err := q.collectionPath()
	if err != nil {
		return err
	}
	item, err = validateIndexableItem(item)
	if err != nil {
		return err
	}
	if err := validateEmbeddingVectors([][]float32{vector}, 1); err != nil {
		return fmt.Errorf("validate Qdrant point vector: %w", err)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("Qdrant embedding model is required")
	}

	body := struct {
		Points []qdrantPoint `json:"points"`
	}{Points: []qdrantPoint{{
		ID: item.ID, Vector: append([]float32(nil), vector...),
		Payload: qdrantPayload{
			UserID: item.UserID, MemoryItemID: item.ID, Kind: item.Kind,
			MemoryKey: item.Key, Value: item.Value, Version: item.Version,
			EmbeddingModel: model,
		},
	}}}
	if _, err := q.doJSON(ctx, http.MethodPut, collectionPath+"/points?wait=true", body, nil, false); err != nil {
		return fmt.Errorf("upsert Qdrant memory point %d: %w", item.ID, err)
	}
	return nil
}

func (q *QdrantIndexer) Delete(ctx context.Context, itemID int64) error {
	collectionPath, err := q.collectionPath()
	if err != nil {
		return err
	}
	if itemID <= 0 {
		return fmt.Errorf("Qdrant memory item ID must be positive: %d", itemID)
	}
	body := struct {
		Points []int64 `json:"points"`
	}{Points: []int64{itemID}}
	if _, err := q.doJSON(ctx, http.MethodPost, collectionPath+"/points/delete?wait=true", body, nil, false); err != nil {
		return fmt.Errorf("delete Qdrant memory point %d: %w", itemID, err)
	}
	return nil
}

func (q *QdrantIndexer) Search(ctx context.Context, userID string, kind Kind, vector []float32, limit int) ([]SearchResult, error) {
	collectionPath, err := q.collectionPath()
	if err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("Qdrant search user ID is required")
	}
	if err := validateEmbeddingVectors([][]float32{vector}, 1); err != nil {
		return nil, fmt.Errorf("validate Qdrant query vector: %w", err)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("Qdrant search limit must be positive: %d", limit)
	}

	filters := []qdrantMatchCondition{{Key: "user_id", Match: qdrantMatch{Value: userID}}}
	if strings.TrimSpace(string(kind)) != "" {
		kind, err = normalizeKind(kind)
		if err != nil {
			return nil, err
		}
		filters = append(filters, qdrantMatchCondition{Key: "kind", Match: qdrantMatch{Value: string(kind)}})
	}
	body := qdrantQueryRequest{
		Query: append([]float32(nil), vector...), Filter: qdrantFilter{Must: filters},
		Limit: limit, WithPayload: true, WithVector: false,
	}
	var response qdrantQueryResponse
	if _, err := q.doJSON(ctx, http.MethodPost, collectionPath+"/points/query", body, &response, false); err != nil {
		return nil, fmt.Errorf("query Qdrant memory points: %w", err)
	}

	results := make([]SearchResult, 0, len(response.Result.Points))
	for index, point := range response.Result.Points {
		result, err := qdrantPointToSearchResult(point, userID, kind, index)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func qdrantPointToSearchResult(point qdrantQueryPoint, userID string, kind Kind, index int) (SearchResult, error) {
	pointID, err := decodeQdrantPointID(point.ID)
	if err != nil {
		return SearchResult{}, qdrantDataError(fmt.Errorf("decode Qdrant result %d point ID: %w", index, err))
	}
	payload := point.Payload
	if strings.TrimSpace(payload.UserID) != userID {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d belongs to user %q, want %q", index, payload.UserID, userID))
	}
	if payload.MemoryItemID != pointID || pointID <= 0 {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d point ID %d and memory item ID %d differ", index, pointID, payload.MemoryItemID))
	}
	resultKind, err := normalizeKind(payload.Kind)
	if err != nil {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d kind: %w", index, err))
	}
	key, err := normalizeMemoryKey(payload.MemoryKey)
	if err != nil {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d key: %w", index, err))
	}
	if kind != "" && resultKind != kind {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d kind %q bypassed requested kind %q", index, resultKind, kind))
	}
	if strings.TrimSpace(payload.Value) == "" || payload.Version <= 0 {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d has invalid value or version", index))
	}
	if math.IsNaN(point.Score) || math.IsInf(point.Score, 0) {
		return SearchResult{}, qdrantDataError(fmt.Errorf("Qdrant result %d score is not finite", index))
	}
	return SearchResult{
		ItemID: pointID, Score: point.Score, UserID: userID, Kind: resultKind,
		Key: key, Value: payload.Value, Version: payload.Version,
	}, nil
}

func (q *QdrantIndexer) collectionPath() (string, error) {
	if q == nil || q.client == nil || q.baseURL == "" {
		return "", fmt.Errorf("Qdrant HTTP client and base URL are required")
	}
	if q.collection == "" {
		return "", fmt.Errorf("Qdrant collection is required")
	}
	return "/collections/" + url.PathEscape(q.collection), nil
}

func (q *QdrantIndexer) doJSON(ctx context.Context, method, path string, body, output any, allowNotFound bool) (int, error) {
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode Qdrant request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, requestBody)
	if err != nil {
		return 0, fmt.Errorf("create Qdrant request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := q.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("call Qdrant: %w", err)
	}
	defer response.Body.Close()
	if allowNotFound && response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response.StatusCode, httpStatusError("Qdrant request", response)
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			return response.StatusCode, qdrantDataError(fmt.Errorf("decode Qdrant response: %w", err))
		}
	}
	return response.StatusCode, nil
}

func validateIndexableItem(item Item) (Item, error) {
	if item.ID <= 0 {
		return Item{}, fmt.Errorf("Qdrant memory item ID must be positive: %d", item.ID)
	}
	item.UserID = strings.TrimSpace(item.UserID)
	if item.UserID == "" {
		return Item{}, fmt.Errorf("Qdrant memory item user ID is required")
	}
	if item.Status != StatusActive {
		return Item{}, fmt.Errorf("Qdrant only indexes active memory items, got %q", item.Status)
	}
	if item.Version <= 0 {
		return Item{}, fmt.Errorf("Qdrant memory item version must be positive: %d", item.Version)
	}
	kind, err := normalizeKind(item.Kind)
	if err != nil {
		return Item{}, err
	}
	key, err := normalizeMemoryKey(item.Key)
	if err != nil {
		return Item{}, err
	}
	item.Kind = kind
	item.Key = key
	item.Value = strings.TrimSpace(item.Value)
	if item.Value == "" {
		return Item{}, fmt.Errorf("Qdrant memory item value is required")
	}
	return item, nil
}

func decodeQdrantPointID(raw json.RawMessage) (int64, error) {
	var id int64
	if err := json.Unmarshal(raw, &id); err != nil {
		return 0, err
	}
	return id, nil
}

type qdrantVectorConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type qdrantCollectionResponse struct {
	Result struct {
		Config struct {
			Params struct {
				Vectors qdrantVectorConfig `json:"vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

type qdrantPayload struct {
	UserID         string `json:"user_id"`
	MemoryItemID   int64  `json:"memory_item_id"`
	Kind           Kind   `json:"kind"`
	MemoryKey      string `json:"memory_key"`
	Value          string `json:"value"`
	Version        int64  `json:"version"`
	EmbeddingModel string `json:"embedding_model"`
}

type qdrantPoint struct {
	ID      int64         `json:"id"`
	Vector  []float32     `json:"vector"`
	Payload qdrantPayload `json:"payload"`
}

type qdrantMatch struct {
	Value string `json:"value"`
}

type qdrantMatchCondition struct {
	Key   string      `json:"key"`
	Match qdrantMatch `json:"match"`
}

type qdrantFilter struct {
	Must []qdrantMatchCondition `json:"must"`
}

type qdrantQueryRequest struct {
	Query       []float32    `json:"query"`
	Filter      qdrantFilter `json:"filter"`
	Limit       int          `json:"limit"`
	WithPayload bool         `json:"with_payload"`
	WithVector  bool         `json:"with_vector"`
}

type qdrantQueryResponse struct {
	Result struct {
		Points []qdrantQueryPoint `json:"points"`
	} `json:"result"`
}

type qdrantQueryPoint struct {
	ID      json.RawMessage `json:"id"`
	Score   float64         `json:"score"`
	Payload qdrantPayload   `json:"payload"`
}
