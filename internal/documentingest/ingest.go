package documentingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type IngestRequest struct {
	Document         Document
	Policy           ChunkPolicy
	ParserVersion    string
	TargetCollection string
	EmbeddingModel   string
	BatchSize        int
}

type IngestResult struct {
	VersionID     int64
	ChunkCount    int
	EmbedBatches  int
	UpsertBatches int
	Noop          bool
}

type IngestionService struct {
	Store    ManifestStore
	Index    VectorIndex
	Embedder BatchEmbedder
	Counter  TokenCounter
}

func (s *IngestionService) Ingest(ctx context.Context, request IngestRequest) (result IngestResult, err error) {
	if s == nil || s.Store == nil || s.Index == nil || s.Embedder == nil || s.Counter == nil {
		return IngestResult{}, fmt.Errorf("ingestion store, index, embedder, and token counter are required")
	}
	document, err := NormalizeDocument(request.Document)
	if err != nil {
		return IngestResult{}, err
	}
	request.EmbeddingModel = strings.TrimSpace(request.EmbeddingModel)
	if request.EmbeddingModel == "" {
		return IngestResult{}, fmt.Errorf("embedding model is required")
	}
	policyHash, err := ChunkPolicyHash(ChunkPolicyIdentity{Format: document.Format, ParserVersion: request.ParserVersion, MaxTokens: request.Policy.MaxTokens, OverlapLines: request.Policy.OverlapLines, EmbeddingModel: request.EmbeddingModel})
	if err != nil {
		return IngestResult{}, err
	}
	build := BuildIdentity{KnowledgeScope: document.KnowledgeScope, DocumentID: document.DocumentID, SourceRef: document.SourceRef, ContentHash: ContentHash(document.Content), ParserVersion: strings.TrimSpace(request.ParserVersion), ChunkPolicyHash: policyHash, TargetCollection: strings.TrimSpace(request.TargetCollection)}
	if err := validateBuildIdentity(build); err != nil {
		return IngestResult{}, err
	}
	if request.BatchSize <= 0 {
		return IngestResult{}, fmt.Errorf("batch_size must be positive: %d", request.BatchSize)
	}

	version, err := s.Store.FindOrCreateVersion(ctx, build)
	if err != nil {
		return IngestResult{}, fmt.Errorf("find or create document version: %w", err)
	}
	result.VersionID = version.ID
	if version.Status == StatusReady || version.Status == StatusActive {
		result.Noop = true
		result.ChunkCount = version.ChunkCount
		return result, nil
	}
	if err := s.Store.ClaimBuild(ctx, version.ID); err != nil {
		return result, fmt.Errorf("claim document build: %w", err)
	}
	failed := true
	defer func() {
		if !failed || err == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if markErr := s.Store.MarkFailed(cleanupCtx, version.ID, err.Error()); markErr != nil {
			err = errors.Join(err, fmt.Errorf("mark document build failed: %w", markErr))
		}
	}()

	chunks, err := ChunkDocument(document, request.Policy, s.Counter)
	if err != nil {
		return result, fmt.Errorf("chunk document: %w", err)
	}
	result.ChunkCount = len(chunks)
	manifests := make([]ChunkManifest, 0, len(chunks))
	dimension := 0
	for start := 0; start < len(chunks); start += request.BatchSize {
		end := start + request.BatchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		texts := make([]string, end-start)
		for i := start; i < end; i++ {
			texts[i-start] = chunks[i].Text
		}
		vectors, embedErr := s.Embedder.Embed(ctx, request.EmbeddingModel, texts)
		if embedErr != nil {
			err = fmt.Errorf("embed chunk batch %d: %w", result.EmbedBatches, embedErr)
			return result, err
		}
		result.EmbedBatches++
		if len(vectors) != len(texts) {
			err = fmt.Errorf("embedding batch returned %d vectors, want %d", len(vectors), len(texts))
			return result, err
		}
		points := make([]VectorPoint, len(vectors))
		for i, vector := range vectors {
			if vectorErr := validateFiniteVector(vector); vectorErr != nil {
				err = fmt.Errorf("embedding %d in batch: %w", i, vectorErr)
				return result, err
			}
			if dimension == 0 {
				dimension = len(vector)
			} else if len(vector) != dimension {
				err = fmt.Errorf("embedding dimension %d, want %d", len(vector), dimension)
				return result, err
			}
			chunk := chunks[start+i]
			pointID := deterministicDocumentPointID(document.KnowledgeScope, chunk.ChunkID)
			points[i] = VectorPoint{ID: pointID, Vector: append([]float32(nil), vector...), Payload: VectorPayload{KnowledgeScope: document.KnowledgeScope, DocumentID: document.DocumentID, ChunkID: chunk.ChunkID, StructureKind: chunk.StructureKind, HeadingPath: chunk.HeadingPath, SourceRef: document.SourceRef, Text: chunk.Text, ContentHash: chunk.ContentHash, EmbeddingModel: request.EmbeddingModel}}
			manifests = append(manifests, ChunkManifest{ChunkID: chunk.ChunkID, StructureKind: chunk.StructureKind, HeadingPath: chunk.HeadingPath, Ordinal: chunk.Ordinal, ContentHash: chunk.ContentHash, TokenCount: chunk.TokenCount, QdrantPointID: pointID})
		}
		if result.UpsertBatches == 0 {
			if err = s.Index.EnsureCollection(ctx, build.TargetCollection, dimension); err != nil {
				err = fmt.Errorf("ensure document collection: %w", err)
				return result, err
			}
		}
		if err = s.Index.UpsertBatch(ctx, build.TargetCollection, points); err != nil {
			err = fmt.Errorf("upsert chunk batch %d: %w", result.UpsertBatches, err)
			return result, err
		}
		result.UpsertBatches++
	}
	if err = s.Store.SaveReadyManifest(ctx, version.ID, manifests); err != nil {
		err = fmt.Errorf("save ready document manifest: %w", err)
		return result, err
	}
	failed = false
	return result, nil
}

func deterministicDocumentPointID(scope, chunkID string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + chunkID))
	value := append([]byte(nil), sum[:16]...)
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	raw := hex.EncodeToString(value)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
