// Package world 定义 gateway 全链路共用的配置、请求/响应契约，以及内部知识块结构。
// 这一层不依赖任何实现细节，相当于「世界规则」——HTTP 层和编排层都引用这里的类型。
package world

// Config 是 App 启动时的全局配置，由 cmd/rag-gateway/main.go 传入。
type Config struct {
	LogDir              string  // 对话 JSONL 日志目录，启动时会 os.MkdirAll 递归创建
	DocDir              string  // ingest 时原文落盘目录，启动时会 os.MkdirAll 递归创建
	RetrievalTopK       int     // 检索最多返回几条命中（默认 5）
	ScoreThreshold      float64 // 相似度低于此阈值的 chunk 会被过滤（默认 0.1）
	PromptMaxChunks     int     // 进入 prompt 的 chunk 数量上限（默认 4）
	PromptMaxChars      int     // 单条 chunk 文本截断长度、prompt 拼接时的字符预算（默认 1200）
	ChatModel           string  // 记录到日志里的聊天模型名（当前 mock，未来可接 Ollama）
	EmbeddingModel      string  // 记录到日志里的 embedding 模型名（当前 mock）
	KnowledgeCollection string  // 知识集合名称，预留字段，未来接 Qdrant collection 时用
}

// IngestRequest 是 POST /ingest 和 POST /debug/split 的请求体。
type IngestRequest struct {
	DocumentID string   `json:"document_id"` // 文档唯一 ID，也用于生成 chunk_id 前缀和落盘文件名
	Title      string   `json:"title"`       // 文档标题，会写入每个 chunk 的 Title 字段
	SourceRef  string   `json:"source_ref"`  // 来源引用，如原始文件名，便于追溯
	Text       string   `json:"text"`        // 待导入的正文，chunker 会按行/标题切分
	Tags       []string `json:"tags"`        // 标签，会复制到每个 chunk（当前检索未使用，预留扩展）
}

// IngestResponse 是 POST /ingest 的成功响应。
type IngestResponse struct {
	DocumentID     string `json:"document_id"`     // 回显导入的文档 ID
	ChunkCount     int    `json:"chunk_count"`     // 本次切出了多少个 chunk 并写入 store
	EmbeddingModel string `json:"embedding_model"` // 当前配置的 embedding 模型名（mock 阶段仅回显）
	Status         string `json:"status"`          // 固定为 "ok" 表示成功
}

// ChatRequest 是 POST /chat 的请求体。
type ChatRequest struct {
	SessionID    string `json:"session_id"`    // 会话 ID，必填，用于日志关联多轮对话
	UserID       string `json:"user_id"`       // 用户 ID，必填，用于日志
	Question     string `json:"question"`      // 用户问题，必填
	Model        string `json:"model"`         // 可选，覆盖默认 ChatModel 记入日志
	UseKnowledge bool   `json:"use_knowledge"` // true 时走检索；false 时跳过检索直接生成
}

// RetrievedChunk 是对外暴露的「命中片段」摘要，不含完整正文。
type RetrievedChunk struct {
	DocumentID string  `json:"document_id"` // 所属文档 ID
	ChunkID    string  `json:"chunk_id"`    // chunk 唯一 ID，格式如 refund-policy#0
	Title      string  `json:"title"`       // chunk 标题（可能含章节名）
	SourceRef  string  `json:"source_ref"`  // 来源引用
	Score      float64 `json:"score"`       // 检索相似度分数，保留 4 位小数
}

// ChatResponse 是 POST /chat 的成功响应。
type ChatResponse struct {
	Answer          string           `json:"answer"`           // 生成器返回的最终回答
	UsedKnowledge   bool             `json:"used_knowledge"`   // 是否有 chunk 进入生成上下文
	RetrievedChunks []RetrievedChunk `json:"retrieved_chunks"` // 实际参与生成的命中列表
	LatencyMS       int64            `json:"latency_ms"`       // 本次 Chat 耗时（毫秒）
}

// DebugRetrievalResponse 是 GET /debug/retrieval 的响应，用于观察检索中间结果。
type DebugRetrievalResponse struct {
	Question           string     `json:"question"`            // 原始问题
	NormalizedQuestion string     `json:"normalized_question"` // 标准化后的问题（小写、去多余空白）
	Hits               []DebugHit `json:"hits"`                // 按分数降序的命中列表
}

// DebugHit 是调试接口里的单条命中，比 RetrievedChunk 多一个 Preview 字段。
type DebugHit struct {
	DocumentID string  `json:"document_id"`
	ChunkID    string  `json:"chunk_id"`
	Title      string  `json:"title"`
	SourceRef  string  `json:"source_ref"`
	Score      float64 `json:"score"`
	Preview    string  `json:"preview"` // 正文前 120 字符预览，方便 curl 查看
}

// SplitPreviewResponse 是 POST /debug/split 的响应，只切分不入库。
type SplitPreviewResponse struct {
	DocumentID string             `json:"document_id"`
	Chunks     []SplitPreviewItem `json:"chunks"`
}

// SplitPreviewItem 是切分预览中的单个 chunk。
type SplitPreviewItem struct {
	ChunkID    string `json:"chunk_id"`    // 如 guide#0
	ChunkIndex int    `json:"chunk_index"` // 文档内从 0 递增的序号
	Text       string `json:"text"`        // 该 chunk 的正文
}

// PromptPreviewResponse 是 GET /debug/prompt 的响应，展示压缩后拼出的 prompt。
type PromptPreviewResponse struct {
	Question       string           `json:"question"`
	SelectedChunks []RetrievedChunk `json:"selected_chunks"` // 压缩后选中的 chunk
	Prompt         string           `json:"prompt"`          // 完整 prompt 字符串
}

// KnowledgeChunk 是存储层和检索层的内部知识单元，比 HTTP 类型多 Tags 和完整 Text。
type KnowledgeChunk struct {
	DocumentID string   // 所属文档
	ChunkID    string   // 全局唯一，格式 document_id#index
	ChunkIndex int      // 文档内序号
	Title      string   // 文档标题 ± 章节标题
	SourceRef  string   // 来源
	Text       string   // chunk 正文
	Tags       []string // 标签副本
}

// RetrievalHit 是一次检索命中的结果：嵌入 KnowledgeChunk 并附带相似度 Score。
type RetrievalHit struct {
	KnowledgeChunk        // 匿名嵌入，可直接访问 DocumentID、Text 等字段
	Score          float64 // 与 query 的相似度，越高越相关
}

// RetrievalResult 是 Retriever.Retrieve 的返回值，包含标准化前后的问题和命中列表。
type RetrievalResult struct {
	Question           string         // 原始问题
	NormalizedQuestion string         // 标准化后用于检索的问题
	Hits               []RetrievalHit // 命中列表，已按分数排序并截断 topK
}
