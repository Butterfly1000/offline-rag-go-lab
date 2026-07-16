package contextretrieval

// Source preserves which retrieval system produced a hit. Ownership and
// filtering rules differ between personal memory and shared documents.
type Source string

const (
	SourceMemory   Source = "memory"
	SourceDocument Source = "document"
)

// Hit is the common prompt-facing shape. Source-specific ownership fields are
// intentionally retained so callers can enforce isolation after retrieval.
type Hit struct {
	Source         Source            `json:"source"`
	ID             string            `json:"id"`
	Content        string            `json:"content"`
	Score          float64           `json:"score"`
	UserID         string            `json:"user_id,omitempty"`
	KnowledgeScope string            `json:"knowledge_scope,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	Title          string            `json:"title,omitempty"`
	SourceRef      string            `json:"source_ref,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}
