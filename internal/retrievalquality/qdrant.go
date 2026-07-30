package retrievalquality

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RetrievalQualityCollection = "offline_rag_retrieval_quality_lab_v1"
	RetrievalQualityAlias      = "offline_rag_retrieval_quality_lab_active"
	maxQdrantErrorBody         = 2048
)

type InfrastructureError struct{ Err error }

func (e *InfrastructureError) Error() string { return e.Err.Error() }
func (e *InfrastructureError) Unwrap() error { return e.Err }

type IntegrityError struct{ Err error }

func (e *IntegrityError) Error() string { return e.Err.Error() }
func (e *IntegrityError) Unwrap() error { return e.Err }

func IsInfrastructure(err error) bool {
	var target *InfrastructureError
	return errors.As(err, &target)
}

func IsIntegrity(err error) bool {
	var target *IntegrityError
	return errors.As(err, &target)
}

type IndexedChunk struct {
	Chunk          Chunk
	Dense          []float32
	Sparse         SparseVector
	EmbeddingModel string
	EncoderID      string
}

type Qdrant struct {
	baseURL       string
	physical      string
	alias         string
	client        *http.Client
	expectedModel string
	expectedID    string
}

type CollectionState struct {
	Status     string `json:"status"`
	PointCount int    `json:"point_count"`
	VectorSize int    `json:"vector_size"`
}

func NewQdrant(baseURL, physicalCollection, alias string) (*Qdrant, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	physicalCollection = strings.TrimSpace(physicalCollection)
	alias = strings.TrimSpace(alias)
	if baseURL == "" {
		return nil, fmt.Errorf("Qdrant base URL is required")
	}
	if physicalCollection != RetrievalQualityCollection {
		return nil, fmt.Errorf("physical collection must be %q", RetrievalQualityCollection)
	}
	if alias != RetrievalQualityAlias {
		return nil, fmt.Errorf("stable alias must be %q", RetrievalQualityAlias)
	}
	return &Qdrant{
		baseURL: baseURL, physical: physicalCollection, alias: alias,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (q *Qdrant) SetExpectedIdentity(embeddingModel, encoderID string) {
	q.expectedModel = strings.TrimSpace(embeddingModel)
	q.expectedID = strings.TrimSpace(encoderID)
}

func (q *Qdrant) EnsureCollection(ctx context.Context, vectorSize int) error {
	if vectorSize <= 0 {
		return fmt.Errorf("Qdrant dense vector size must be positive")
	}
	path := q.collectionPath(q.physical)
	var existing qdrantCollectionResponse
	status, err := q.doJSON(ctx, http.MethodGet, path, nil, &existing, true)
	if err != nil {
		return &InfrastructureError{Err: fmt.Errorf("inspect retrieval collection: %w", err)}
	}
	if status == http.StatusNotFound {
		body := map[string]any{
			"vectors": map[string]any{
				"dense": map[string]any{"size": vectorSize, "distance": "Cosine"},
			},
			"sparse_vectors": map[string]any{
				"sparse": map[string]any{"modifier": "idf"},
			},
		}
		if _, err := q.doJSON(ctx, http.MethodPut, path, body, nil, false); err != nil {
			return &InfrastructureError{Err: fmt.Errorf("create retrieval collection: %w", err)}
		}
	} else if err := validateQdrantCollection(existing, vectorSize); err != nil {
		return &IntegrityError{Err: err}
	}
	for _, field := range []string{"knowledge_scope", "document_id", "chunk_id"} {
		body := map[string]any{"field_name": field, "field_schema": "keyword"}
		if _, err := q.doJSON(ctx, http.MethodPut, path+"/index?wait=true", body, nil, false); err != nil {
			return &InfrastructureError{Err: fmt.Errorf("create payload index %s: %w", field, err)}
		}
	}
	return nil
}

func (q *Qdrant) Upsert(ctx context.Context, item IndexedChunk) error {
	chunk := normalizeChunk(item.Chunk)
	if chunk.KnowledgeScope == "" || chunk.DocumentID == "" || chunk.ChunkID == "" ||
		chunk.Content == "" || chunk.ContentHash != sha256Hex([]byte(chunk.Content)) {
		return fmt.Errorf("indexed chunk identity, content, or content_hash is invalid")
	}
	if len(item.Dense) == 0 {
		return fmt.Errorf("indexed dense vector is required")
	}
	for index, value := range item.Dense {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("indexed dense value %d must be finite", index)
		}
	}
	if err := validateSparseVector(item.Sparse); err != nil {
		return err
	}
	item.EmbeddingModel = strings.TrimSpace(item.EmbeddingModel)
	item.EncoderID = strings.TrimSpace(item.EncoderID)
	if item.EmbeddingModel == "" || item.EncoderID == "" {
		return fmt.Errorf("embedding model and encoder identity are required")
	}
	payload := qdrantPayload{
		KnowledgeScope: chunk.KnowledgeScope, DocumentID: chunk.DocumentID, ChunkID: chunk.ChunkID,
		Title: chunk.Title, HeadingPath: chunk.HeadingPath, SourceRef: chunk.SourceRef,
		StructureKind: chunk.StructureKind, Content: chunk.Content, ContentHash: chunk.ContentHash,
		EmbeddingModel: item.EmbeddingModel, EncoderID: item.EncoderID,
	}
	body := map[string]any{"points": []any{map[string]any{
		"id": StablePointID(chunk.KnowledgeScope, chunk.ChunkID),
		"vector": map[string]any{
			"dense":  append([]float32(nil), item.Dense...),
			"sparse": item.Sparse,
		},
		"payload": payload,
	}}}
	if _, err := q.doJSON(ctx, http.MethodPut, q.collectionPath(q.physical)+"/points?wait=true", body, nil, false); err != nil {
		return &InfrastructureError{Err: fmt.Errorf("upsert retrieval point: %w", err)}
	}
	return nil
}

func (q *Qdrant) QueryDense(ctx context.Context, scope string, vector []float32, limit int) ([]Candidate, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("dense query vector is required")
	}
	for index, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("dense query value %d must be finite", index)
		}
	}
	return q.query(ctx, scope, "dense", append([]float32(nil), vector...), limit)
}

func (q *Qdrant) QuerySparse(ctx context.Context, scope string, vector SparseVector, limit int) ([]Candidate, error) {
	if err := validateSparseVector(vector); err != nil {
		return nil, err
	}
	return q.query(ctx, scope, "sparse", vector, limit)
}

func (q *Qdrant) query(ctx context.Context, scope, using string, vector any, limit int) ([]Candidate, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" || limit <= 0 {
		return nil, fmt.Errorf("Qdrant query scope and positive limit are required")
	}
	body := map[string]any{
		"query": vector, "using": using,
		"filter": map[string]any{"must": []any{map[string]any{
			"key": "knowledge_scope", "match": map[string]any{"value": scope},
		}}},
		"limit": limit, "with_payload": true, "with_vector": false,
	}
	var response qdrantQueryResponse
	if _, err := q.doJSON(ctx, http.MethodPost, q.collectionPath(q.alias)+"/points/query", body, &response, false); err != nil {
		return nil, &InfrastructureError{Err: fmt.Errorf("query %s retrieval points: %w", using, err)}
	}
	candidates := make([]Candidate, 0, len(response.Result.Points))
	seen := map[string]bool{}
	for index, point := range response.Result.Points {
		candidate, err := q.validatePoint(point, scope)
		if err != nil {
			return nil, &IntegrityError{Err: fmt.Errorf("validate %s point %d: %w", using, index, err)}
		}
		if seen[candidate.ChunkID] {
			return nil, &IntegrityError{Err: fmt.Errorf("duplicate Qdrant chunk_id %q", candidate.ChunkID)}
		}
		seen[candidate.ChunkID] = true
		candidate.Reason = using
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (q *Qdrant) validatePoint(point qdrantQueryPoint, scope string) (Candidate, error) {
	payload := point.Payload
	if strings.TrimSpace(payload.KnowledgeScope) != scope {
		return Candidate{}, fmt.Errorf("point belongs to knowledge_scope %q, want %q", payload.KnowledgeScope, scope)
	}
	if strings.TrimSpace(payload.DocumentID) == "" || strings.TrimSpace(payload.ChunkID) == "" ||
		strings.TrimSpace(payload.Content) == "" {
		return Candidate{}, fmt.Errorf("point payload identity and content are required")
	}
	if point.ID != StablePointID(scope, payload.ChunkID) {
		return Candidate{}, fmt.Errorf("point ID %q does not match stable identity", point.ID)
	}
	if strings.TrimSpace(payload.ContentHash) != sha256Hex([]byte(payload.Content)) {
		return Candidate{}, fmt.Errorf("point content_hash does not match content")
	}
	if strings.TrimSpace(payload.EmbeddingModel) == "" || strings.TrimSpace(payload.EncoderID) == "" {
		return Candidate{}, fmt.Errorf("point embedding model and encoder identity are required")
	}
	if q.expectedModel != "" && payload.EmbeddingModel != q.expectedModel {
		return Candidate{}, fmt.Errorf("embedding model %q does not match expected %q", payload.EmbeddingModel, q.expectedModel)
	}
	if q.expectedID != "" && payload.EncoderID != q.expectedID {
		return Candidate{}, fmt.Errorf("encoder identity %q does not match expected %q", payload.EncoderID, q.expectedID)
	}
	if math.IsNaN(point.Score) || math.IsInf(point.Score, 0) {
		return Candidate{}, fmt.Errorf("point score must be finite")
	}
	return Candidate{
		KnowledgeScope: payload.KnowledgeScope, DocumentID: payload.DocumentID,
		ChunkID: payload.ChunkID, HeadingPath: payload.HeadingPath,
		Content: payload.Content, Score: point.Score,
	}, nil
}

func (q *Qdrant) Activate(ctx context.Context) error {
	var aliases qdrantAliasesResponse
	if _, err := q.doJSON(ctx, http.MethodGet, "/aliases", nil, &aliases, false); err != nil {
		return &InfrastructureError{Err: fmt.Errorf("inspect Qdrant aliases: %w", err)}
	}
	var current string
	for _, alias := range aliases.Result.Aliases {
		if alias.AliasName == q.alias {
			current = alias.CollectionName
			break
		}
	}
	if current == q.physical {
		return nil
	}
	actions := []any{}
	if current != "" {
		actions = append(actions, map[string]any{"delete_alias": map[string]any{"alias_name": q.alias}})
	}
	actions = append(actions, map[string]any{"create_alias": map[string]any{
		"collection_name": q.physical, "alias_name": q.alias,
	}})
	if _, err := q.doJSON(ctx, http.MethodPost, "/collections/aliases", map[string]any{"actions": actions}, nil, false); err != nil {
		return &InfrastructureError{Err: fmt.Errorf("activate retrieval alias: %w", err)}
	}
	return nil
}

func (q *Qdrant) Inspect(ctx context.Context, vectorSize, expectedPoints int) (CollectionState, error) {
	var response qdrantCollectionResponse
	if _, err := q.doJSON(ctx, http.MethodGet, q.collectionPath(q.physical), nil, &response, false); err != nil {
		return CollectionState{}, &InfrastructureError{Err: fmt.Errorf("inspect retrieval collection: %w", err)}
	}
	if err := validateQdrantCollection(response, vectorSize); err != nil {
		return CollectionState{}, &IntegrityError{Err: err}
	}
	if !strings.EqualFold(response.Result.Status, "green") {
		return CollectionState{}, &IntegrityError{Err: fmt.Errorf("retrieval collection status=%q, want green", response.Result.Status)}
	}
	if response.Result.PointsCount != expectedPoints {
		return CollectionState{}, &IntegrityError{Err: fmt.Errorf(
			"retrieval collection points=%d, want %d", response.Result.PointsCount, expectedPoints,
		)}
	}
	return CollectionState{
		Status: response.Result.Status, PointCount: response.Result.PointsCount, VectorSize: vectorSize,
	}, nil
}

func StablePointID(scope, chunkID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(scope) + "\x00" + strings.TrimSpace(chunkID)))
	hexID := hex.EncodeToString(sum[:16])
	return hexID[0:8] + "-" + hexID[8:12] + "-" + hexID[12:16] + "-" + hexID[16:20] + "-" + hexID[20:32]
}

func (q *Qdrant) collectionPath(name string) string {
	return "/collections/" + url.PathEscape(name)
}

func (q *Qdrant) doJSON(ctx context.Context, method, path string, body, output any, allowNotFound bool) (int, error) {
	if q == nil || q.client == nil || q.baseURL == "" {
		return 0, fmt.Errorf("Qdrant client is not initialized")
	}
	var reader io.Reader
	if body != nil {
		content, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode Qdrant request: %w", err)
		}
		reader = bytes.NewReader(content)
	}
	request, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, reader)
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
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		content, _ := io.ReadAll(io.LimitReader(response.Body, maxQdrantErrorBody))
		return response.StatusCode, fmt.Errorf("Qdrant status %d: %s", response.StatusCode, strings.TrimSpace(string(content)))
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			return response.StatusCode, fmt.Errorf("decode Qdrant response: %w", err)
		}
	}
	return response.StatusCode, nil
}

func validateQdrantCollection(response qdrantCollectionResponse, vectorSize int) error {
	dense, exists := response.Result.Config.Params.Vectors["dense"]
	if !exists || dense.Size != vectorSize || !strings.EqualFold(dense.Distance, "Cosine") {
		return fmt.Errorf("retrieval collection dense vector config mismatch")
	}
	sparse, exists := response.Result.Config.Params.SparseVectors["sparse"]
	if !exists || !strings.EqualFold(sparse.Modifier, "idf") {
		return fmt.Errorf("retrieval collection sparse vector config mismatch")
	}
	return nil
}

type qdrantVectorConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type qdrantSparseConfig struct {
	Modifier string `json:"modifier"`
}

type qdrantCollectionResponse struct {
	Result struct {
		Status      string `json:"status"`
		PointsCount int    `json:"points_count"`
		Config      struct {
			Params struct {
				Vectors       map[string]qdrantVectorConfig `json:"vectors"`
				SparseVectors map[string]qdrantSparseConfig `json:"sparse_vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

type qdrantPayload struct {
	KnowledgeScope string `json:"knowledge_scope"`
	DocumentID     string `json:"document_id"`
	ChunkID        string `json:"chunk_id"`
	Title          string `json:"title"`
	HeadingPath    string `json:"heading_path"`
	SourceRef      string `json:"source_ref"`
	StructureKind  string `json:"structure_kind"`
	Content        string `json:"content"`
	ContentHash    string `json:"content_hash"`
	EmbeddingModel string `json:"embedding_model"`
	EncoderID      string `json:"encoder_id"`
}

type qdrantQueryPoint struct {
	ID      string        `json:"id"`
	Score   float64       `json:"score"`
	Payload qdrantPayload `json:"payload"`
}

type qdrantQueryResponse struct {
	Result struct {
		Points []qdrantQueryPoint `json:"points"`
	} `json:"result"`
}

type qdrantAliasesResponse struct {
	Result struct {
		Aliases []struct {
			AliasName      string `json:"alias_name"`
			CollectionName string `json:"collection_name"`
		} `json:"aliases"`
	} `json:"result"`
}
