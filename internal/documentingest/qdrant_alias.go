package documentingest

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (q *QdrantIndex) CollectionInfo(ctx context.Context, name string) (CollectionInfo, error) {
	path, err := q.collectionPath(name)
	if err != nil {
		return CollectionInfo{}, err
	}
	var response struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
			PayloadSchema map[string]any `json:"payload_schema"`
		} `json:"result"`
	}
	if _, err := q.doJSON(ctx, http.MethodGet, path, nil, &response, false); err != nil {
		return CollectionInfo{}, err
	}
	indexes := make(map[string]bool, len(response.Result.PayloadSchema))
	for field := range response.Result.PayloadSchema {
		indexes[field] = true
	}
	return CollectionInfo{VectorSize: response.Result.Config.Params.Vectors.Size, Distance: response.Result.Config.Params.Vectors.Distance, PayloadIndexes: indexes}, nil
}

func (q *QdrantIndex) Count(ctx context.Context, name, scope string) (int, error) {
	path, err := q.collectionPath(name)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(scope) == "" {
		return 0, fmt.Errorf("count scope is required")
	}
	body := map[string]any{"filter": map[string]any{"must": []any{map[string]any{"key": "knowledge_scope", "match": map[string]string{"value": scope}}}}, "exact": true}
	var response struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	if _, err := q.doJSON(ctx, http.MethodPost, path+"/points/count", body, &response, false); err != nil {
		return 0, err
	}
	return response.Result.Count, nil
}

func (q *QdrantIndex) Fetch(ctx context.Context, name string, pointIDs []string) ([]IndexedPoint, error) {
	path, err := q.collectionPath(name)
	if err != nil {
		return nil, err
	}
	if len(pointIDs) == 0 {
		return nil, fmt.Errorf("fetch point IDs are required")
	}
	body := struct {
		IDs         []string `json:"ids"`
		WithPayload bool     `json:"with_payload"`
		WithVector  bool     `json:"with_vector"`
	}{append([]string(nil), pointIDs...), true, false}
	var response struct {
		Result []struct {
			ID      string        `json:"id"`
			Payload VectorPayload `json:"payload"`
		} `json:"result"`
	}
	if _, err := q.doJSON(ctx, http.MethodPost, path+"/points", body, &response, false); err != nil {
		return nil, err
	}
	points := make([]IndexedPoint, len(response.Result))
	for i, point := range response.Result {
		points[i] = IndexedPoint{ID: point.ID, KnowledgeScope: point.Payload.KnowledgeScope, ChunkID: point.Payload.ChunkID, ContentHash: point.Payload.ContentHash}
	}
	return points, nil
}

func (q *QdrantIndex) ResolveAlias(ctx context.Context, alias string) (string, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "", fmt.Errorf("alias is required")
	}
	var response struct {
		Result struct {
			Aliases []struct {
				AliasName      string `json:"alias_name"`
				CollectionName string `json:"collection_name"`
			} `json:"aliases"`
		} `json:"result"`
	}
	_, err := q.doJSON(ctx, http.MethodGet, "/aliases", nil, &response, false)
	if err != nil {
		return "", err
	}
	target := ""
	for _, item := range response.Result.Aliases {
		if item.AliasName != alias {
			continue
		}
		if target != "" {
			return "", fmt.Errorf("alias %q appears more than once", alias)
		}
		target = item.CollectionName
	}
	return target, nil
}

func (q *QdrantIndex) SwitchAlias(ctx context.Context, alias, from, to string) error {
	alias, from, to = strings.TrimSpace(alias), strings.TrimSpace(from), strings.TrimSpace(to)
	if alias == "" || to == "" {
		return fmt.Errorf("alias and target collection are required")
	}
	actions := make([]map[string]map[string]string, 0, 2)
	if from != "" {
		actions = append(actions, map[string]map[string]string{"delete_alias": {"alias_name": alias}})
	}
	actions = append(actions, map[string]map[string]string{"create_alias": {"alias_name": alias, "collection_name": to}})
	if _, err := q.doJSON(ctx, http.MethodPost, "/collections/aliases", struct {
		Actions []map[string]map[string]string `json:"actions"`
	}{actions}, nil, false); err != nil {
		return fmt.Errorf("switch Qdrant alias: %w", err)
	}
	return nil
}

var _ SnapshotIndex = (*QdrantIndex)(nil)
