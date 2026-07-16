package contextretrieval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// DocumentChunk is the fact stored in the derived document vector index.
type DocumentChunk struct {
	KnowledgeScope string
	DocumentID     string
	ChunkID        string // Stable ID unique across the knowledge scope.
	Title          string
	SourceRef      string
	Text           string
}

func normalizeDocumentChunk(chunk DocumentChunk) (DocumentChunk, error) {
	chunk.KnowledgeScope = strings.TrimSpace(chunk.KnowledgeScope)
	chunk.DocumentID = strings.TrimSpace(chunk.DocumentID)
	chunk.ChunkID = strings.TrimSpace(chunk.ChunkID)
	chunk.Title = strings.TrimSpace(chunk.Title)
	chunk.SourceRef = strings.TrimSpace(chunk.SourceRef)
	chunk.Text = strings.TrimSpace(chunk.Text)
	if chunk.KnowledgeScope == "" {
		return DocumentChunk{}, fmt.Errorf("document chunk knowledge_scope is required")
	}
	if chunk.DocumentID == "" {
		return DocumentChunk{}, fmt.Errorf("document chunk document_id is required")
	}
	if chunk.ChunkID == "" {
		return DocumentChunk{}, fmt.Errorf("document chunk chunk_id is required")
	}
	if chunk.Text == "" {
		return DocumentChunk{}, fmt.Errorf("document chunk text is required")
	}
	return chunk, nil
}

// DeterministicDocumentPointID returns a stable UUID accepted by Qdrant. Scope
// participates in the identity so equal chunk IDs in two knowledge bases do not collide.
func DeterministicDocumentPointID(knowledgeScope, chunkID string) (string, error) {
	knowledgeScope = strings.TrimSpace(knowledgeScope)
	chunkID = strings.TrimSpace(chunkID)
	if knowledgeScope == "" {
		return "", fmt.Errorf("document point knowledge_scope is required")
	}
	if chunkID == "" {
		return "", fmt.Errorf("document point chunk_id is required")
	}
	sum := sha256.Sum256([]byte(knowledgeScope + "\x00" + chunkID))
	bytes := append([]byte(nil), sum[:16]...)
	// Use the UUID v4 layout and RFC 4122 variant while keeping deterministic bytes.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(bytes)
	return raw[0:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32], nil
}

func documentContentHash(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}
