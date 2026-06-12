package world

type Config struct {
	LogDir              string
	DocDir              string
	RetrievalTopK       int
	ScoreThreshold      float64
	PromptMaxChunks     int
	PromptMaxChars      int
	ChatModel           string
	EmbeddingModel      string
	KnowledgeCollection string
}

type IngestRequest struct {
	DocumentID string   `json:"document_id"`
	Title      string   `json:"title"`
	SourceRef  string   `json:"source_ref"`
	Text       string   `json:"text"`
	Tags       []string `json:"tags"`
}

type IngestResponse struct {
	DocumentID     string `json:"document_id"`
	ChunkCount     int    `json:"chunk_count"`
	EmbeddingModel string `json:"embedding_model"`
	Status         string `json:"status"`
}

type ChatRequest struct {
	SessionID    string `json:"session_id"`
	UserID       string `json:"user_id"`
	Question     string `json:"question"`
	Model        string `json:"model"`
	UseKnowledge bool   `json:"use_knowledge"`
}

type RetrievedChunk struct {
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id"`
	Title      string  `json:"title"`
	SourceRef  string  `json:"source_ref"`
	Score      float64 `json:"score"`
}

type ChatResponse struct {
	Answer          string           `json:"answer"`
	UsedKnowledge   bool             `json:"used_knowledge"`
	RetrievedChunks []RetrievedChunk `json:"retrieved_chunks"`
	LatencyMS       int64            `json:"latency_ms"`
}

type DebugRetrievalResponse struct {
	Question           string     `json:"question"`
	NormalizedQuestion string     `json:"normalized_question"`
	Hits               []DebugHit `json:"hits"`
}

type DebugHit struct {
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id"`
	Title      string  `json:"title"`
	SourceRef  string  `json:"source_ref"`
	Score      float64 `json:"score"`
	Preview    string  `json:"preview"`
}

type SplitPreviewResponse struct {
	DocumentID string             `json:"document_id"`
	Chunks     []SplitPreviewItem `json:"chunks"`
}

type SplitPreviewItem struct {
	ChunkID    string `json:"chunk_id"`
	ChunkIndex int    `json:"chunk_index"`
	Text       string `json:"text"`
}

type PromptPreviewResponse struct {
	Question       string           `json:"question"`
	SelectedChunks []RetrievedChunk `json:"selected_chunks"`
	Prompt         string           `json:"prompt"`
}

type KnowledgeChunk struct {
	DocumentID string
	ChunkID    string
	ChunkIndex int
	Title      string
	SourceRef  string
	Text       string
	Tags       []string
}

type RetrievalHit struct {
	KnowledgeChunk
	Score float64
}

type RetrievalResult struct {
	Question           string
	NormalizedQuestion string
	Hits               []RetrievalHit
}
