package documentingest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

type MySQLManifestStore struct{ db *sql.DB }

func NewMySQLManifestStore(db *sql.DB) *MySQLManifestStore { return &MySQLManifestStore{db: db} }

func (s *MySQLManifestStore) FindOrCreateVersion(ctx context.Context, build BuildIdentity) (Version, error) {
	if s == nil || s.db == nil {
		return Version{}, fmt.Errorf("MySQL manifest database is required")
	}
	if err := validateBuildIdentity(build); err != nil {
		return Version{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO document_sources
        (knowledge_scope, document_id, source_ref)
        VALUES (?, ?, ?)
        ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), source_ref = VALUES(source_ref)`, build.KnowledgeScope, build.DocumentID, build.SourceRef)
	if err != nil {
		return Version{}, fmt.Errorf("upsert document source: %w", err)
	}
	sourceID, err := positiveLastInsertID(result, "document source")
	if err != nil {
		return Version{}, err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO document_versions
        (document_source_id, content_hash, parser_version, chunk_policy_hash, status, target_collection)
        VALUES (?, ?, ?, ?, 'pending', ?)
        ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id)`, sourceID, build.ContentHash, build.ParserVersion, build.ChunkPolicyHash, build.TargetCollection)
	if err != nil {
		return Version{}, fmt.Errorf("upsert document version: %w", err)
	}
	versionID, err := positiveLastInsertID(result, "document version")
	if err != nil {
		return Version{}, err
	}
	var status string
	var chunkCount int
	if err := tx.QueryRowContext(ctx, `SELECT status, chunk_count FROM document_versions WHERE id = ? AND document_source_id = ?`, versionID, sourceID).Scan(&status, &chunkCount); err != nil {
		return Version{}, fmt.Errorf("read document version: %w", err)
	}
	version := Version{ID: versionID, DocumentSourceID: sourceID, Status: VersionStatus(status), ChunkCount: chunkCount}
	if !version.Status.Valid() {
		return Version{}, fmt.Errorf("database returned invalid document status %q", status)
	}
	if err := tx.Commit(); err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *MySQLManifestStore) ClaimBuild(ctx context.Context, versionID int64) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("MySQL manifest database is required")
	}
	if versionID <= 0 {
		return fmt.Errorf("document version ID must be positive")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE document_versions SET status = 'building', error_message = '' WHERE id = ? AND status IN ('pending', 'failed')`, versionID)
	if err != nil {
		return err
	}
	return requireOneAffected(result, "claim document build")
}

func (s *MySQLManifestStore) SaveReadyManifest(ctx context.Context, versionID int64, chunks []ChunkManifest) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("MySQL manifest database is required")
	}
	if versionID <= 0 || len(chunks) == 0 {
		return fmt.Errorf("document version and chunk manifest are required")
	}
	for i, chunk := range chunks {
		if err := validateChunkManifest(chunk, i); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM document_chunk_manifests WHERE document_version_id = ?`, versionID); err != nil {
		return fmt.Errorf("clear document manifest: %w", err)
	}
	for _, chunk := range chunks {
		_, err := tx.ExecContext(ctx, `INSERT INTO document_chunk_manifests
            (document_version_id, chunk_id, structure_kind, heading_path, ordinal, content_hash, token_count, qdrant_point_id)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, versionID, chunk.ChunkID, chunk.StructureKind, chunk.HeadingPath, chunk.Ordinal, chunk.ContentHash, chunk.TokenCount, chunk.QdrantPointID)
		if err != nil {
			return fmt.Errorf("insert chunk manifest ordinal %d: %w", chunk.Ordinal, err)
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE document_versions SET status = 'ready', chunk_count = ?, error_message = '' WHERE id = ? AND status = 'building'`, len(chunks), versionID)
	if err != nil {
		return err
	}
	if err := requireOneAffected(result, "mark document version ready"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQLManifestStore) MarkFailed(ctx context.Context, versionID int64, reason string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("MySQL manifest database is required")
	}
	if versionID <= 0 {
		return fmt.Errorf("document version ID must be positive")
	}
	reason = boundDocumentBuildError(reason)
	result, err := s.db.ExecContext(ctx, `UPDATE document_versions SET status = 'failed', error_message = ? WHERE id = ? AND status = 'building'`, reason, versionID)
	if err != nil {
		return err
	}
	return requireOneAffected(result, "mark document build failed")
}

func validateBuildIdentity(build BuildIdentity) error {
	if _, err := normalizeIdentifier("knowledge_scope", build.KnowledgeScope); err != nil {
		return err
	}
	if _, err := normalizeIdentifier("document_id", build.DocumentID); err != nil {
		return err
	}
	if _, err := normalizeSourceRef(build.SourceRef); err != nil {
		return err
	}
	if _, err := normalizeIdentifier("parser_version", build.ParserVersion); err != nil {
		return err
	}
	if len(build.ContentHash) != 64 || len(build.ChunkPolicyHash) != 64 {
		return fmt.Errorf("content and chunk policy hashes must be SHA256 hex")
	}
	for _, hash := range []string{build.ContentHash, build.ChunkPolicyHash} {
		for _, char := range hash {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return fmt.Errorf("build hash must be lowercase hexadecimal")
			}
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(build.TargetCollection), documentIngestionCollectionPrefix) {
		return fmt.Errorf("target collection must use %q prefix", documentIngestionCollectionPrefix)
	}
	if len(build.TargetCollection) > 255 {
		return fmt.Errorf("target collection exceeds 255 bytes")
	}
	return nil
}

func validateChunkManifest(chunk ChunkManifest, index int) error {
	if len(chunk.ChunkID) != 64 || len(chunk.ContentHash) != 64 || strings.TrimSpace(chunk.StructureKind) == "" || strings.TrimSpace(chunk.HeadingPath) == "" || strings.TrimSpace(chunk.QdrantPointID) == "" || chunk.Ordinal < 0 || chunk.TokenCount <= 0 {
		return fmt.Errorf("chunk manifest %d is invalid", index)
	}
	return nil
}

func requireOneAffected(result sql.Result, operation string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if count != 1 {
		return fmt.Errorf("%s affected %d rows, want 1", operation, count)
	}
	return nil
}

func positiveLastInsertID(result sql.Result, entity string) (int64, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read %s ID: %w", entity, err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("read %s ID: driver returned %d", entity, id)
	}
	return id, nil
}

func boundDocumentBuildError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "document build failed"
	}
	const limit = 2048
	const suffix = "...[truncated]"
	if len(value) <= limit {
		return value
	}
	value = value[:limit-len(suffix)]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + suffix
}

var _ ManifestStore = (*MySQLManifestStore)(nil)
