package documentingest

import "context"

const documentIngestionCollectionPrefix = "offline_rag_document_ingestion_lab_"

type BuildIdentity struct {
	KnowledgeScope   string
	DocumentID       string
	SourceRef        string
	ContentHash      string
	ParserVersion    string
	ChunkPolicyHash  string
	TargetCollection string
}

type Version struct {
	ID               int64
	DocumentSourceID int64
	Status           VersionStatus
	ChunkCount       int
}

type ChunkManifest struct {
	ChunkID       string
	StructureKind string
	HeadingPath   string
	Ordinal       int
	ContentHash   string
	TokenCount    int
	QdrantPointID string
}

type ManifestStore interface {
	FindOrCreateVersion(ctx context.Context, build BuildIdentity) (Version, error)
	ClaimBuild(ctx context.Context, versionID int64) error
	SaveReadyManifest(ctx context.Context, versionID int64, chunks []ChunkManifest) error
	MarkFailed(ctx context.Context, versionID int64, reason string) error
}

type VectorPayload struct {
	KnowledgeScope string `json:"knowledge_scope"`
	DocumentID     string `json:"document_id"`
	ChunkID        string `json:"chunk_id"`
	StructureKind  string `json:"structure_kind"`
	HeadingPath    string `json:"heading_path"`
	SourceRef      string `json:"source_ref"`
	Text           string `json:"text"`
	ContentHash    string `json:"content_hash"`
	EmbeddingModel string `json:"embedding_model"`
}

type VectorPoint struct {
	ID      string        `json:"id"`
	Vector  []float32     `json:"vector"`
	Payload VectorPayload `json:"payload"`
}

type VectorIndex interface {
	EnsureCollection(ctx context.Context, name string, vectorSize int) error
	UpsertBatch(ctx context.Context, name string, points []VectorPoint) error
	DeletePoints(ctx context.Context, name string, pointIDs []string) error
}

type BatchEmbedder interface {
	Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}
