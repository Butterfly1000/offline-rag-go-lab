package documentingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type CollectionInfo struct {
	VectorSize     int
	Distance       string
	PayloadIndexes map[string]bool
}
type IndexedPoint struct{ ID, KnowledgeScope, ChunkID, ContentHash string }

type SnapshotIndex interface {
	CollectionInfo(context.Context, string) (CollectionInfo, error)
	Count(context.Context, string, string) (int, error)
	Fetch(context.Context, string, []string) ([]IndexedPoint, error)
	ResolveAlias(context.Context, string) (string, error)
	SwitchAlias(context.Context, string, string, string) error
}

type PublicationStore interface {
	ActivateVersion(context.Context, int64, int64) error
}

type SnapshotVersion struct{ DocumentSourceID, VersionID int64 }

type SnapshotManifest struct {
	VersionID, DocumentSourceID                int64
	KnowledgeScope, Collection, ManifestDigest string
	Chunks                                     []ChunkManifest
	Versions                                   []SnapshotVersion
}
type VerifyRequest struct {
	Snapshot           SnapshotManifest
	ExpectedVectorSize int
}
type VerificationReport struct {
	Collection, KnowledgeScope, ManifestDigest string
	VerifiedAt                                 time.Time
	PointCount                                 int
}
type ActivateRequest struct {
	Alias, From  string
	Snapshot     SnapshotManifest
	Verification VerificationReport
}
type ActivationResult struct {
	AliasSwitched, MySQLActivated, ReconciliationRequired bool
	From, To                                              string
}
type RollbackRequest struct{ Alias, From, To string }

type Publisher struct {
	Index              SnapshotIndex
	Store              PublicationStore
	Now                func() time.Time
	MaxVerificationAge time.Duration
}

func ManifestDigest(chunks []ChunkManifest) string {
	ordered := append([]ChunkManifest(nil), chunks...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].QdrantPointID != ordered[j].QdrantPointID {
			return ordered[i].QdrantPointID < ordered[j].QdrantPointID
		}
		return ordered[i].ChunkID < ordered[j].ChunkID
	})
	hash := sha256.New()
	for _, chunk := range ordered {
		fmt.Fprintf(hash, "%d\x00%s\x00%s\x00%s\x00%d\x00", chunk.Ordinal, chunk.ChunkID, chunk.ContentHash, chunk.QdrantPointID, chunk.TokenCount)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (p *Publisher) Verify(ctx context.Context, request VerifyRequest) (VerificationReport, error) {
	if p == nil || p.Index == nil {
		return VerificationReport{}, fmt.Errorf("snapshot index is required")
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return VerificationReport{}, err
	}
	if request.ExpectedVectorSize <= 0 {
		return VerificationReport{}, fmt.Errorf("expected vector size must be positive")
	}
	actualDigest := ManifestDigest(request.Snapshot.Chunks)
	if request.Snapshot.ManifestDigest != actualDigest {
		return VerificationReport{}, fmt.Errorf("snapshot manifest digest does not match chunks")
	}
	info, err := p.Index.CollectionInfo(ctx, request.Snapshot.Collection)
	if err != nil {
		return VerificationReport{}, err
	}
	if info.VectorSize != request.ExpectedVectorSize || !strings.EqualFold(info.Distance, "Cosine") {
		return VerificationReport{}, fmt.Errorf("snapshot vector config size=%d distance=%s", info.VectorSize, info.Distance)
	}
	for _, field := range []string{"knowledge_scope", "document_id", "chunk_id"} {
		if !info.PayloadIndexes[field] {
			return VerificationReport{}, fmt.Errorf("snapshot lacks payload index %s", field)
		}
	}
	count, err := p.Index.Count(ctx, request.Snapshot.Collection, request.Snapshot.KnowledgeScope)
	if err != nil {
		return VerificationReport{}, err
	}
	if count != len(request.Snapshot.Chunks) {
		return VerificationReport{}, fmt.Errorf("snapshot scope count=%d, manifest=%d", count, len(request.Snapshot.Chunks))
	}
	ids := make([]string, len(request.Snapshot.Chunks))
	expected := make(map[string]ChunkManifest, len(ids))
	for i, chunk := range request.Snapshot.Chunks {
		ids[i] = chunk.QdrantPointID
		if _, exists := expected[chunk.QdrantPointID]; exists {
			return VerificationReport{}, fmt.Errorf("duplicate manifest point ID %s", chunk.QdrantPointID)
		}
		expected[chunk.QdrantPointID] = chunk
	}
	points, err := p.Index.Fetch(ctx, request.Snapshot.Collection, ids)
	if err != nil {
		return VerificationReport{}, err
	}
	if len(points) != len(ids) {
		return VerificationReport{}, fmt.Errorf("fetched points=%d, manifest=%d", len(points), len(ids))
	}
	seen := make(map[string]bool, len(points))
	for _, point := range points {
		chunk, ok := expected[point.ID]
		if !ok || seen[point.ID] {
			return VerificationReport{}, fmt.Errorf("unexpected or duplicate indexed point %s", point.ID)
		}
		seen[point.ID] = true
		if point.KnowledgeScope != request.Snapshot.KnowledgeScope || point.ChunkID != chunk.ChunkID || point.ContentHash != chunk.ContentHash {
			return VerificationReport{}, fmt.Errorf("indexed point %s does not match manifest", point.ID)
		}
	}
	return VerificationReport{Collection: request.Snapshot.Collection, KnowledgeScope: request.Snapshot.KnowledgeScope, ManifestDigest: actualDigest, VerifiedAt: p.now(), PointCount: count}, nil
}

func (p *Publisher) Activate(ctx context.Context, request ActivateRequest) (ActivationResult, error) {
	result := ActivationResult{From: request.From, To: request.Snapshot.Collection}
	if p == nil || p.Index == nil || p.Store == nil {
		return result, fmt.Errorf("publisher index and store are required")
	}
	if err := validateSnapshot(request.Snapshot); err != nil {
		return result, err
	}
	if request.Verification.Collection != request.Snapshot.Collection || request.Verification.KnowledgeScope != request.Snapshot.KnowledgeScope || request.Verification.ManifestDigest != request.Snapshot.ManifestDigest || request.Verification.PointCount != len(request.Snapshot.Chunks) {
		return result, fmt.Errorf("verification report does not match snapshot")
	}
	age := p.now().Sub(request.Verification.VerifiedAt)
	maxAge := p.MaxVerificationAge
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	if age < 0 || age > maxAge {
		return result, fmt.Errorf("verification report is stale")
	}
	current, err := p.Index.ResolveAlias(ctx, request.Alias)
	if err != nil {
		return result, err
	}
	if current != request.From {
		return result, fmt.Errorf("alias currently points to %q, want %q", current, request.From)
	}
	if err := p.Index.SwitchAlias(ctx, request.Alias, request.From, request.Snapshot.Collection); err != nil {
		return result, err
	}
	result.AliasSwitched = true
	for _, version := range snapshotVersions(request.Snapshot) {
		if err := p.Store.ActivateVersion(ctx, version.DocumentSourceID, version.VersionID); err != nil {
			result.ReconciliationRequired = true
			return result, fmt.Errorf("alias switched but MySQL activation requires reconciliation: %w", err)
		}
	}
	result.MySQLActivated = true
	return result, nil
}

func (p *Publisher) Rollback(ctx context.Context, request RollbackRequest) error {
	if p == nil || p.Index == nil {
		return fmt.Errorf("snapshot index is required")
	}
	if err := validateLabCollection(request.From); err != nil {
		return err
	}
	if err := validateLabCollection(request.To); err != nil {
		return err
	}
	current, err := p.Index.ResolveAlias(ctx, request.Alias)
	if err != nil {
		return err
	}
	if current != request.From {
		return fmt.Errorf("alias currently points to %q, want %q", current, request.From)
	}
	return p.Index.SwitchAlias(ctx, request.Alias, request.From, request.To)
}

func validateSnapshot(snapshot SnapshotManifest) error {
	if len(snapshotVersions(snapshot)) == 0 || strings.TrimSpace(snapshot.KnowledgeScope) == "" || len(snapshot.Chunks) == 0 {
		return fmt.Errorf("snapshot identity and chunks are required")
	}
	if err := validateLabCollection(snapshot.Collection); err != nil {
		return err
	}
	if len(snapshot.ManifestDigest) != 64 {
		return fmt.Errorf("snapshot manifest digest is required")
	}
	return nil
}
func snapshotVersions(snapshot SnapshotManifest) []SnapshotVersion {
	if len(snapshot.Versions) > 0 {
		return snapshot.Versions
	}
	if snapshot.VersionID > 0 && snapshot.DocumentSourceID > 0 {
		return []SnapshotVersion{{DocumentSourceID: snapshot.DocumentSourceID, VersionID: snapshot.VersionID}}
	}
	return nil
}
func validateLabCollection(value string) error {
	if !strings.HasPrefix(strings.TrimSpace(value), documentIngestionCollectionPrefix) {
		return fmt.Errorf("collection must use %q prefix", documentIngestionCollectionPrefix)
	}
	return nil
}
func (p *Publisher) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}
