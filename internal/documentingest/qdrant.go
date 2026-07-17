package documentingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type QdrantIndex struct {
	baseURL string
	client  *http.Client
}

func NewQdrantIndex(baseURL string) *QdrantIndex {
	return &QdrantIndex{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), client: &http.Client{Timeout: 30 * time.Second}}
}

func (q *QdrantIndex) EnsureCollection(ctx context.Context, name string, vectorSize int) error {
	path, err := q.collectionPath(name)
	if err != nil {
		return err
	}
	if vectorSize <= 0 {
		return fmt.Errorf("Qdrant vector size must be positive: %d", vectorSize)
	}
	var current struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	status, err := q.doJSON(ctx, http.MethodGet, path, nil, &current, true)
	if err != nil {
		return fmt.Errorf("inspect Qdrant collection: %w", err)
	}
	if status == http.StatusNotFound {
		body := map[string]any{"vectors": map[string]any{"size": vectorSize, "distance": "Cosine"}}
		if _, err := q.doJSON(ctx, http.MethodPut, path, body, nil, false); err != nil {
			return fmt.Errorf("create Qdrant collection: %w", err)
		}
	} else if current.Result.Config.Params.Vectors.Size != vectorSize || !strings.EqualFold(current.Result.Config.Params.Vectors.Distance, "Cosine") {
		return fmt.Errorf("Qdrant collection %q vector config is size=%d distance=%s, want size=%d distance=Cosine", name, current.Result.Config.Params.Vectors.Size, current.Result.Config.Params.Vectors.Distance, vectorSize)
	}
	for _, field := range []string{"knowledge_scope", "document_id", "chunk_id"} {
		body := map[string]string{"field_name": field, "field_schema": "keyword"}
		if _, err := q.doJSON(ctx, http.MethodPut, path+"/index?wait=true", body, nil, false); err != nil {
			return fmt.Errorf("create Qdrant payload index %s: %w", field, err)
		}
	}
	return nil
}

func (q *QdrantIndex) UpsertBatch(ctx context.Context, name string, points []VectorPoint) error {
	path, err := q.collectionPath(name)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return fmt.Errorf("Qdrant upsert points are required")
	}
	for i, point := range points {
		if strings.TrimSpace(point.ID) == "" || strings.TrimSpace(point.Payload.KnowledgeScope) == "" || strings.TrimSpace(point.Payload.ChunkID) == "" || strings.TrimSpace(point.Payload.Text) == "" {
			return fmt.Errorf("Qdrant point %d identity and text are required", i)
		}
		if err := validateFiniteVector(point.Vector); err != nil {
			return fmt.Errorf("Qdrant point %d: %w", i, err)
		}
	}
	if _, err := q.doJSON(ctx, http.MethodPut, path+"/points?wait=true", struct {
		Points []VectorPoint `json:"points"`
	}{Points: points}, nil, false); err != nil {
		return fmt.Errorf("upsert Qdrant points: %w", err)
	}
	return nil
}

func (q *QdrantIndex) DeletePoints(ctx context.Context, name string, pointIDs []string) error {
	path, err := q.collectionPath(name)
	if err != nil {
		return err
	}
	if len(pointIDs) == 0 {
		return fmt.Errorf("Qdrant point IDs are required")
	}
	ids := append([]string(nil), pointIDs...)
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
		if ids[i] == "" {
			return fmt.Errorf("Qdrant point ID %d is empty", i)
		}
	}
	if _, err := q.doJSON(ctx, http.MethodPost, path+"/points/delete?wait=true", struct {
		Points []string `json:"points"`
	}{Points: ids}, nil, false); err != nil {
		return fmt.Errorf("delete Qdrant points: %w", err)
	}
	return nil
}

func (q *QdrantIndex) collectionPath(name string) (string, error) {
	if q == nil || q.client == nil || q.baseURL == "" {
		return "", fmt.Errorf("Qdrant HTTP client and base URL are required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("Qdrant collection is required")
	}
	return "/collections/" + url.PathEscape(name), nil
}

func (q *QdrantIndex) doJSON(ctx context.Context, method, path string, body, output any, allowNotFound bool) (int, error) {
	reader := bytes.NewReader(nil)
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := q.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if allowNotFound && response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 2049))
		if len(detail) > 2048 {
			detail = append(detail[:2048], []byte("...[truncated]")...)
		}
		return response.StatusCode, fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(detail)))
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}

func validateFiniteVector(vector []float32) error {
	if len(vector) == 0 {
		return fmt.Errorf("vector is required")
	}
	for i, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("vector value %d is not finite", i)
		}
	}
	return nil
}
