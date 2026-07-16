package contextretrieval

import (
	"bytes"
	"context"
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

const maxDocumentHTTPErrorBodyBytes = 2048

type DocumentQdrant struct {
	baseURL    string
	collection string
	client     *http.Client
}

func NewDocumentQdrant(baseURL, collection string) *DocumentQdrant {
	return &DocumentQdrant{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		collection: strings.TrimSpace(collection),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (q *DocumentQdrant) EnsureCollection(ctx context.Context, vectorSize int) error {
	path, err := q.collectionPath()
	if err != nil {
		return err
	}
	if vectorSize <= 0 {
		return fmt.Errorf("document Qdrant vector size must be positive: %d", vectorSize)
	}

	var existing documentCollectionResponse
	status, err := q.doJSON(ctx, http.MethodGet, path, nil, &existing, true)
	if err != nil {
		return classifyDocumentRequestError("inspect collection", err)
	}
	if status == http.StatusNotFound {
		body := struct {
			Vectors documentVectorConfig `json:"vectors"`
		}{Vectors: documentVectorConfig{Size: vectorSize, Distance: "Cosine"}}
		if _, err := q.doJSON(ctx, http.MethodPut, path, body, nil, false); err != nil {
			return classifyDocumentRequestError("create collection", err)
		}
	} else {
		vectors := existing.Result.Config.Params.Vectors
		if vectors.Size != vectorSize || !strings.EqualFold(vectors.Distance, "Cosine") {
			return IntegrityFailure(SourceDocument, fmt.Errorf(
				"document Qdrant collection %q uses size=%d distance=%s, want size=%d distance=Cosine",
				q.collection, vectors.Size, vectors.Distance, vectorSize,
			))
		}
	}

	for _, field := range []string{"knowledge_scope", "document_id"} {
		body := struct {
			FieldName   string `json:"field_name"`
			FieldSchema string `json:"field_schema"`
		}{FieldName: field, FieldSchema: "keyword"}
		if _, err := q.doJSON(ctx, http.MethodPut, path+"/index?wait=true", body, nil, false); err != nil {
			return classifyDocumentRequestError("create payload index "+field, err)
		}
	}
	return nil
}

func (q *DocumentQdrant) Upsert(ctx context.Context, chunk DocumentChunk, vector []float32, embeddingModel string) error {
	path, err := q.collectionPath()
	if err != nil {
		return err
	}
	chunk, err = normalizeDocumentChunk(chunk)
	if err != nil {
		return err
	}
	if err := validateDocumentVector(vector); err != nil {
		return err
	}
	embeddingModel = strings.TrimSpace(embeddingModel)
	if embeddingModel == "" {
		return fmt.Errorf("document embedding model is required")
	}
	pointID, err := DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		return err
	}
	payload := documentPayload{
		KnowledgeScope: chunk.KnowledgeScope, DocumentID: chunk.DocumentID, ChunkID: chunk.ChunkID,
		Title: chunk.Title, SourceRef: chunk.SourceRef, Text: chunk.Text,
		ContentHash: documentContentHash(chunk.Text), EmbeddingModel: embeddingModel,
	}
	body := struct {
		Points []documentPoint `json:"points"`
	}{Points: []documentPoint{{ID: pointID, Vector: append([]float32(nil), vector...), Payload: payload}}}
	if _, err := q.doJSON(ctx, http.MethodPut, path+"/points?wait=true", body, nil, false); err != nil {
		return classifyDocumentRequestError("upsert point", err)
	}
	return nil
}

func (q *DocumentQdrant) Search(ctx context.Context, knowledgeScope string, vector []float32, limit int) ([]Hit, error) {
	path, err := q.collectionPath()
	if err != nil {
		return nil, err
	}
	knowledgeScope = strings.TrimSpace(knowledgeScope)
	if knowledgeScope == "" {
		return nil, fmt.Errorf("document search knowledge_scope is required")
	}
	if err := validateDocumentVector(vector); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, fmt.Errorf("document search limit must be positive: %d", limit)
	}
	body := documentQueryRequest{
		Query: append([]float32(nil), vector...),
		Filter: documentFilter{Must: []documentMatchCondition{{
			Key: "knowledge_scope", Match: documentMatch{Value: knowledgeScope},
		}}},
		Limit: limit, WithPayload: true, WithVector: false,
	}
	var response documentQueryResponse
	if _, err := q.doJSON(ctx, http.MethodPost, path+"/points/query", body, &response, false); err != nil {
		return nil, classifyDocumentRequestError("query points", err)
	}

	hits := make([]Hit, 0, len(response.Result.Points))
	for index, point := range response.Result.Points {
		hit, err := documentPointToHit(point, knowledgeScope)
		if err != nil {
			return nil, IntegrityFailure(SourceDocument, fmt.Errorf("validate result %d: %w", index, err))
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func documentPointToHit(point documentQueryPoint, knowledgeScope string) (Hit, error) {
	payload := point.Payload
	if strings.TrimSpace(payload.KnowledgeScope) != knowledgeScope {
		return Hit{}, fmt.Errorf("document result belongs to knowledge_scope %q, want %q", payload.KnowledgeScope, knowledgeScope)
	}
	chunk, err := normalizeDocumentChunk(DocumentChunk{
		KnowledgeScope: payload.KnowledgeScope, DocumentID: payload.DocumentID, ChunkID: payload.ChunkID,
		Title: payload.Title, SourceRef: payload.SourceRef, Text: payload.Text,
	})
	if err != nil {
		return Hit{}, err
	}
	wantPointID, err := DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		return Hit{}, err
	}
	if strings.TrimSpace(point.ID) != wantPointID {
		return Hit{}, fmt.Errorf("document point ID %q does not match payload identity %q", point.ID, wantPointID)
	}
	if strings.TrimSpace(payload.ContentHash) != documentContentHash(chunk.Text) {
		return Hit{}, fmt.Errorf("document content_hash does not match text")
	}
	if strings.TrimSpace(payload.EmbeddingModel) == "" {
		return Hit{}, fmt.Errorf("document embedding_model is required")
	}
	hit, err := ValidateHit(Hit{
		Source: SourceDocument, ID: "document:" + wantPointID, Content: chunk.Text,
		Score: point.Score, KnowledgeScope: chunk.KnowledgeScope,
		Title: chunk.Title, SourceRef: chunk.SourceRef,
		Metadata: map[string]string{
			"document_id": chunk.DocumentID, "chunk_id": chunk.ChunkID,
			"content_hash": payload.ContentHash, "embedding_model": payload.EmbeddingModel,
		},
	})
	if err != nil {
		return Hit{}, err
	}
	return hit, nil
}

func (q *DocumentQdrant) collectionPath() (string, error) {
	if q == nil || q.client == nil || q.baseURL == "" {
		return "", fmt.Errorf("document Qdrant HTTP client and base URL are required")
	}
	if q.collection == "" {
		return "", fmt.Errorf("document Qdrant collection is required")
	}
	return "/collections/" + url.PathEscape(q.collection), nil
}

func (q *DocumentQdrant) doJSON(ctx context.Context, method, path string, body, output any, allowNotFound bool) (int, error) {
	requestBody := bytes.NewReader(nil)
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode document Qdrant request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, requestBody)
	if err != nil {
		return 0, fmt.Errorf("create document Qdrant request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := q.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("call document Qdrant: %w", err)
	}
	defer response.Body.Close()
	if allowNotFound && response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response.StatusCode, documentHTTPStatusError(response)
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			return response.StatusCode, &documentResponseDataError{Err: fmt.Errorf("decode document Qdrant response: %w", err)}
		}
	}
	return response.StatusCode, nil
}

func classifyDocumentRequestError(operation string, err error) error {
	var dataErr *documentResponseDataError
	if errors.As(err, &dataErr) {
		return IntegrityFailure(SourceDocument, fmt.Errorf("%s: %w", operation, err))
	}
	return InfrastructureFailure(SourceDocument, fmt.Errorf("%s: %w", operation, err))
}

func validateDocumentVector(vector []float32) error {
	if len(vector) == 0 {
		return fmt.Errorf("document vector is required")
	}
	for index, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("document vector value %d must be finite", index)
		}
	}
	return nil
}

func documentHTTPStatusError(response *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxDocumentHTTPErrorBodyBytes+1))
	if readErr != nil {
		return fmt.Errorf("document Qdrant failed: status %d", response.StatusCode)
	}
	truncated := len(body) > maxDocumentHTTPErrorBodyBytes
	if truncated {
		body = body[:maxDocumentHTTPErrorBodyBytes]
	}
	detail := strings.TrimSpace(string(body))
	if truncated {
		detail += " ...[truncated]"
	}
	if detail == "" {
		return fmt.Errorf("document Qdrant failed: status %d", response.StatusCode)
	}
	return fmt.Errorf("document Qdrant failed: status %d: %s", response.StatusCode, detail)
}

type documentResponseDataError struct{ Err error }

func (e *documentResponseDataError) Error() string { return e.Err.Error() }
func (e *documentResponseDataError) Unwrap() error { return e.Err }

type documentVectorConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

type documentCollectionResponse struct {
	Result struct {
		Config struct {
			Params struct {
				Vectors documentVectorConfig `json:"vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

type documentPayload struct {
	KnowledgeScope string `json:"knowledge_scope"`
	DocumentID     string `json:"document_id"`
	ChunkID        string `json:"chunk_id"`
	Title          string `json:"title,omitempty"`
	SourceRef      string `json:"source_ref,omitempty"`
	Text           string `json:"text"`
	ContentHash    string `json:"content_hash"`
	EmbeddingModel string `json:"embedding_model"`
}

type documentPoint struct {
	ID      string          `json:"id"`
	Vector  []float32       `json:"vector"`
	Payload documentPayload `json:"payload"`
}

type documentMatch struct {
	Value string `json:"value"`
}

type documentMatchCondition struct {
	Key   string        `json:"key"`
	Match documentMatch `json:"match"`
}

type documentFilter struct {
	Must []documentMatchCondition `json:"must"`
}

type documentQueryRequest struct {
	Query       []float32      `json:"query"`
	Filter      documentFilter `json:"filter"`
	Limit       int            `json:"limit"`
	WithPayload bool           `json:"with_payload"`
	WithVector  bool           `json:"with_vector"`
}

type documentQueryPoint struct {
	ID      string          `json:"id"`
	Score   float64         `json:"score"`
	Payload documentPayload `json:"payload"`
}

type documentQueryResponse struct {
	Result struct {
		Points []documentQueryPoint `json:"points"`
	} `json:"result"`
}
