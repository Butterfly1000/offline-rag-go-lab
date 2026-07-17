// Package documentingest owns production document identity, chunking, indexing,
// publication, and retrieval-evaluation behavior used by the teaching project.
package documentingest

type DocumentFormat string

const (
	FormatMarkdown DocumentFormat = "markdown"
	FormatGo       DocumentFormat = "go"
)

// Document identifies one logical source independently from its machine path.
// Content is normalized before hashing so line endings do not create fake versions.
type Document struct {
	KnowledgeScope string
	DocumentID     string
	SourceRef      string
	Format         DocumentFormat
	Content        []byte
}

// ChunkIdentityInput contains only properties that should rename a chunk.
// A document version and global line number are intentionally absent.
type ChunkIdentityInput struct {
	KnowledgeScope   string
	DocumentID       string
	StructureKind    string
	HeadingPath      string
	Content          string
	DuplicateOrdinal int
}

// ChunkPolicyIdentity contains every setting that changes chunk vectors. The
// existing database column is named chunk_policy_hash, but the hash also binds
// the embedding model so incompatible vector spaces cannot share a build.
type ChunkPolicyIdentity struct {
	Format         DocumentFormat
	ParserVersion  string
	MaxTokens      int
	OverlapLines   int
	EmbeddingModel string
}
