package main

import (
	"context"       // 给 MySQL、Qdrant、Ollama 请求设置统一超时，避免真实外部服务卡住整个 demo。
	"database/sql"  // Go 标准数据库接口；这里用它 ping MySQL，后续 ingest 会用 MySQL 保存文档版本和 chunk manifest。
	"encoding/json" // 解析 Qdrant HTTP 返回的 JSON，例如 /collections 的 collection 列表。
	"errors"        // 构造简单错误，例如 HTTP 返回空 body 时给出明确错误。
	"flag"          // 解析 check 子命令的 --config、--chat-model、--timeout 等手动参数。
	"fmt"           // 打印教学友好的步骤输出，也用于包装错误上下文。
	"io"            // 读取 HTTP body，以及把 usage 输出到 stdout/stderr。
	"log"           // 关闭默认时间前缀，让命令行输出更像一步步教学记录。
	"net/http"      // 直接访问 Qdrant HTTP API；Ollama/MySQL 客户端内部也会走网络。
	"net/url"       // 对 Qdrant collection 名称做 URL 转义，避免特殊字符破坏路径。
	"os"            // 读取命令行参数、选择 stdout/stderr，并在失败时用非 0 状态码退出。
	"path/filepath" // 从 --source 推导默认 document_id，并检查文件扩展名。
	"strings"       // 清理配置值、拼接 vector 预览，以及根据错误文本给出排障提示。
	"time"          // 表达 check 的整体超时和 HTTP client 超时。
	"unicode"       // 把文件名清理成安全的 document_id 时判断字母和数字。
	"unicode/utf8"  // 按 rune 截断 chunk preview，避免把中文字符切坏。

	_ "github.com/go-sql-driver/mysql"           // 注册 MySQL driver；database/sql.Open("mysql", dsn) 依赖这个空导入。
	"offline-rag-go-lab/internal/documentingest" // 复用真实文档 ingestion 的 Qwen tokenizer 计数器。
	"offline-rag-go-lab/internal/fileconfig"     // 复用项目里的 KEY=VALUE 配置读取方式。
	"offline-rag-go-lab/internal/memoryitem"     // 复用真实 Ollama /api/embed 客户端，检查 bge-m3 embedding。
	"offline-rag-go-lab/internal/recentchat"     // 复用真实 Ollama /api/show 客户端，检查 qwen:7b 元数据。
)

const (
	defaultConfigPath         = "config/recent-chat.env"
	defaultChatModel          = "qwen:7b"
	defaultEmbeddingModel     = "bge-m3"
	defaultOllamaBaseURL      = "http://127.0.0.1:11434"
	defaultQdrantBaseURL      = "http://127.0.0.1:6333"
	defaultDocumentCollection = "offline_rag_document_chunks_v1"
	defaultTokenizerPath      = "assets/tokenizers/qwen2/tokenizer.json"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		if err := runCheck(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "\ncheck failed: %v\n", err)
			printFailureHint(os.Stderr, err)
			os.Exit(1)
		}
	case "chunk":
		if err := runChunk(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "\nchunk failed: %v\n", err)
			printFailureHint(os.Stderr, err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printUsage(os.Stdout)
	default:
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  go run ./cmd/rag-real-loop-demo check [flags]")
	fmt.Fprintln(w, "  go run ./cmd/rag-real-loop-demo chunk [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "check 参数:")
	fmt.Fprintln(w, "  --config path       本地 KEY=VALUE 配置文件，默认 config/recent-chat.env")
	fmt.Fprintln(w, "  --chat-model name   要检查的 Ollama 生成模型，默认 qwen:7b")
	fmt.Fprintln(w, "  --timeout duration  本次检查的总超时时间，默认 20s")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "chunk 参数:")
	fmt.Fprintln(w, "  --config path       本地 KEY=VALUE 配置文件，默认 config/recent-chat.env")
	fmt.Fprintln(w, "  --source path       仓库内的源文档路径，必填，例如 docs/teaching/recent-window-layer-01.md")
	fmt.Fprintln(w, "  --format name       文档格式：markdown 或 go；不填时按扩展名推导")
	fmt.Fprintln(w, "  --scope name        knowledge_scope，默认 rag-real-loop")
	fmt.Fprintln(w, "  --document-id name  逻辑文档 ID；不填时从文件名推导")
	fmt.Fprintln(w, "  --max-tokens n      每个 chunk 的 token 上限，默认 160")
	fmt.Fprintln(w, "  --overlap-lines n   超长结构切分时保留的重叠行数，默认 2")
}

// runCheck 是第一阶段唯一实现的子命令。
//
// 设计目标：
// 1. 只检查真实依赖，不写 MySQL、不写 Qdrant、不调用 qwen:7b 生成回答。
// 2. 按真实 RAG 闭环顺序检查：配置 -> tokenizer -> MySQL -> Qdrant -> 生成模型 -> embedding。
// 3. 每一步都打印新人能读懂的状态，为后续 chunk / ingest / search / answer 铺路。
func runCheck(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultConfigPath, "本地 KEY=VALUE 配置文件")
	chatModel := flags.String("chat-model", defaultChatModel, "要检查的 Ollama 生成模型")
	timeout := flags.Duration("timeout", 20*time.Second, "本次检查的总超时时间")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}

	// 一个总 context 贯穿所有外部探测。真实系统里所有跨进程调用都应该有超时，
	// 否则 MySQL、Qdrant 或 Ollama 卡住时，业务请求也会无限等待。
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("== 真实 RAG 小闭环依赖检查 ==")
	fmt.Println("本命令只检查真实依赖；不会写入知识库，不会检索，也不会调用模型生成回答。")
	fmt.Println()

	cfg, err := loadCheckConfig(*configPath, *chatModel)
	if err != nil {
		return err
	}
	printConfigSummary(*configPath, cfg)

	// 第一阶段故意保持只读：在写文档、写向量之前，先确认每个真实依赖都能访问。
	if err := checkTokenizer(cfg.TokenizerPath); err != nil {
		return err
	}
	if err := checkMySQL(ctx, cfg.MySQLDSN); err != nil {
		return err
	}
	if err := checkQdrant(ctx, cfg.QdrantBaseURL, cfg.DocumentCollection); err != nil {
		return err
	}
	if err := checkOllamaChatModel(cfg.OllamaBaseURL, cfg.ChatModel); err != nil {
		return err
	}
	if err := checkOllamaEmbedding(ctx, cfg.OllamaBaseURL, cfg.EmbeddingModel); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("OK: 第一阶段需要的真实 RAG 依赖都可访问。")
	return nil
}

// runChunk 是真实小闭环的第二步。
//
// 这一阶段只回答一个问题：一份真实知识文档进入 RAG 系统前，会被切成哪些 chunk？
// 它会使用真实 tokenizer 做 token 计数，也会使用 production document ingestion 里的
// ChunkDocument 逻辑，但不会写 MySQL、不会写 Qdrant、不会调用 Ollama。
func runChunk(args []string) error {
	flags := flag.NewFlagSet("chunk", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultConfigPath, "本地 KEY=VALUE 配置文件")
	sourcePath := flags.String("source", "", "仓库内的源文档路径")
	formatName := flags.String("format", "", "文档格式：markdown 或 go；不填时按扩展名推导")
	scope := flags.String("scope", "rag-real-loop", "knowledge_scope")
	documentID := flags.String("document-id", "", "逻辑文档 ID；不填时从文件名推导")
	maxTokens := flags.Int("max-tokens", 160, "每个 chunk 的 token 上限")
	overlapLines := flags.Int("overlap-lines", 2, "超长结构切分时保留的重叠行数")
	if err := flags.Parse(args); err != nil {
		return err
	}

	source := strings.TrimSpace(*sourcePath)
	if source == "" {
		return fmt.Errorf("--source is required")
	}
	if filepath.IsAbs(source) {
		return fmt.Errorf("--source must be repository-relative, got %s", source)
	}
	if cleaned := filepath.Clean(source); cleaned != source || cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return fmt.Errorf("--source must stay inside the repository and use a clean relative path, got %s", source)
	}
	if *documentID == "" {
		*documentID = defaultDocumentIDFromSource(source)
	}
	format, err := resolveDocumentFormat(source, *formatName)
	if err != nil {
		return err
	}

	fmt.Println("== 真实 RAG 小闭环：文档分块 ==")
	fmt.Println("本命令只读取源文档并展示 chunk；不会写入 MySQL/Qdrant，也不会调用模型生成回答。")
	fmt.Println()

	cfg, err := loadTokenizerConfig(*configPath)
	if err != nil {
		return err
	}
	printChunkConfigSummary(*configPath, cfg.TokenizerPath)

	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read source %s: %w", source, err)
	}
	counter, err := documentingest.NewQwenTokenCounter(cfg.TokenizerPath)
	if err != nil {
		return fmt.Errorf("load tokenizer %s: %w", cfg.TokenizerPath, err)
	}
	fullTokens, err := counter.Count(string(content))
	if err != nil {
		return fmt.Errorf("count source tokens: %w", err)
	}

	document := documentingest.Document{
		KnowledgeScope: *scope,
		DocumentID:     *documentID,
		SourceRef:      source,
		Format:         format,
		Content:        content,
	}
	policy := documentingest.ChunkPolicy{MaxTokens: *maxTokens, OverlapLines: *overlapLines}
	chunks, err := documentingest.ChunkDocument(document, policy, counter)
	if err != nil {
		return err
	}

	printSourceSummary(source, format, *scope, *documentID, len(content), fullTokens, policy)
	printChunkSummary(chunks)
	return nil
}

type checkConfig struct {
	MySQLDSN           string // MySQL 是结构化事实源：后续保存 document source、version、chunk manifest。
	OllamaBaseURL      string // Ollama 同时承载生成模型和 embedding 模型，本地默认端口是 11434。
	EmbeddingModel     string // embedding 模型把 chunk/question 变成向量，项目默认使用 bge-m3。
	ChatModel          string // 生成模型读取检索上下文并回答，当前实验默认检查 qwen:7b。
	QdrantBaseURL      string // Qdrant 是向量检索库，本地默认端口是 6333。
	DocumentCollection string // 文档 chunk 写入/检索的 Qdrant collection。
	TokenizerPath      string // Qwen tokenizer 资产路径，用于真实 token 计数和后续 chunk 上限。
}

type tokenizerConfig struct {
	TokenizerPath string
}

// loadTokenizerConfig 只读取 chunk 阶段真正需要的配置：tokenizer 路径。
//
// chunk 是纯本地读文件 + token 计数，所以不需要 MySQL、Qdrant、Ollama。
// 这能帮助学习者明确区分：“分块”发生在向量化和入库之前。
func loadTokenizerConfig(path string) (tokenizerConfig, error) {
	values, err := fileconfig.Load(path)
	if err != nil {
		return tokenizerConfig{}, err
	}
	return tokenizerConfig{
		TokenizerPath: valueOrDefault(values, "RECENT_CHAT_TOKENIZER_PATH", defaultTokenizerPath),
	}, nil
}

// loadCheckConfig 读取项目统一的本地配置文件。
//
// 这里故意复用 recent-chat/document-ingestion 已经使用的 config/recent-chat.env，
// 而不是给 demo 单独发明一套配置。原因是：真实实战里最怕“检查的是 A 环境，
// 运行的是 B 环境”。统一配置可以避免 MySQL、Qdrant、Ollama 指向不同实例。
func loadCheckConfig(path, chatModel string) (checkConfig, error) {
	values, err := fileconfig.Load(path)
	if err != nil {
		return checkConfig{}, err
	}
	mysqlDSN, err := fileconfig.Required(values, "RECENT_CHAT_MYSQL_DSN")
	if err != nil {
		return checkConfig{}, err
	}
	cfg := checkConfig{
		MySQLDSN:           mysqlDSN,
		OllamaBaseURL:      valueOrDefault(values, "OLLAMA_BASE_URL", defaultOllamaBaseURL),
		EmbeddingModel:     valueOrDefault(values, "OLLAMA_EMBED_MODEL", defaultEmbeddingModel),
		ChatModel:          strings.TrimSpace(chatModel),
		QdrantBaseURL:      valueOrDefault(values, "QDRANT_BASE_URL", defaultQdrantBaseURL),
		DocumentCollection: valueOrDefault(values, "QDRANT_DOCUMENT_COLLECTION", defaultDocumentCollection),
		TokenizerPath:      valueOrDefault(values, "RECENT_CHAT_TOKENIZER_PATH", defaultTokenizerPath),
	}
	if cfg.ChatModel == "" {
		return checkConfig{}, fmt.Errorf("--chat-model is required")
	}
	return cfg, nil
}

// valueOrDefault 表示“配置优先，常见本地默认兜底”。
//
// 对新人来说，配置文件只要写必要项就能先跑起来；对实战来说，显式配置仍然永远胜出。
func valueOrDefault(values map[string]string, key, fallback string) string {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	return value
}

func printConfigSummary(path string, cfg checkConfig) {
	// DSN 通常包含数据库密码，所以只确认“已配置”，绝不把原文打印到终端。
	fmt.Println("[1/6] 配置")
	fmt.Printf("  file: %s\n", path)
	fmt.Println("  RECENT_CHAT_MYSQL_DSN: configured (redacted)")
	fmt.Printf("  OLLAMA_BASE_URL: %s\n", cfg.OllamaBaseURL)
	fmt.Printf("  OLLAMA_EMBED_MODEL: %s\n", cfg.EmbeddingModel)
	fmt.Printf("  chat model: %s\n", cfg.ChatModel)
	fmt.Printf("  QDRANT_BASE_URL: %s\n", cfg.QdrantBaseURL)
	fmt.Printf("  QDRANT_DOCUMENT_COLLECTION: %s\n", cfg.DocumentCollection)
	fmt.Printf("  RECENT_CHAT_TOKENIZER_PATH: %s\n", cfg.TokenizerPath)
	fmt.Println("  状态: OK")
	fmt.Println()
}

func printChunkConfigSummary(path, tokenizerPath string) {
	fmt.Println("[1/3] 配置")
	fmt.Printf("  file: %s\n", path)
	fmt.Printf("  RECENT_CHAT_TOKENIZER_PATH: %s\n", tokenizerPath)
	fmt.Println("  本阶段不读取 MySQL/Qdrant/Ollama 配置")
	fmt.Println("  状态: OK")
	fmt.Println()
}

// checkTokenizer 检查真实 tokenizer 资产。
//
// mock 版本可以按字符或简单分词切文本；生产 RAG 更关心“会不会超过模型上下文”。
// 因此后续 chunk 命令会用同一个 tokenizer 统计 token，并按 token 上限切块。
func checkTokenizer(path string) error {
	fmt.Println("[2/6] Tokenizer")
	counter, err := documentingest.NewQwenTokenCounter(path)
	if err != nil {
		return fmt.Errorf("load tokenizer %s: %w", path, err)
	}
	tokens, err := counter.Count("真实 RAG 闭环检查")
	if err != nil {
		return fmt.Errorf("count tokenizer probe text: %w", err)
	}
	fmt.Printf("  path: %s\n", path)
	fmt.Printf("  探测文本 token 数: %d\n", tokens)
	fmt.Println("  状态: OK")
	fmt.Println()
	return nil
}

func printSourceSummary(source string, format documentingest.DocumentFormat, scope, documentID string, bytesLen, fullTokens int, policy documentingest.ChunkPolicy) {
	fmt.Println("[2/3] 源文档")
	fmt.Printf("  source_ref: %s\n", source)
	fmt.Printf("  format: %s\n", format)
	fmt.Printf("  knowledge_scope: %s\n", scope)
	fmt.Printf("  document_id: %s\n", documentID)
	fmt.Printf("  原始字节数: %d\n", bytesLen)
	fmt.Printf("  整篇 token 数: %d\n", fullTokens)
	fmt.Printf("  chunk policy: max_tokens=%d overlap_lines=%d\n", policy.MaxTokens, policy.OverlapLines)
	fmt.Println("  状态: OK")
	fmt.Println()
}

func printChunkSummary(chunks []documentingest.Chunk) {
	fmt.Println("[3/3] Chunk 结果")
	fmt.Printf("  chunk 数量: %d\n", len(chunks))
	for _, chunk := range chunks {
		fmt.Println()
		fmt.Printf("  [%d]\n", chunk.Ordinal)
		fmt.Printf("    kind: %s\n", chunk.StructureKind)
		fmt.Printf("    heading_path: %s\n", chunk.HeadingPath)
		fmt.Printf("    token_count: %d\n", chunk.TokenCount)
		fmt.Printf("    chunk_id: %s\n", chunk.ChunkID)
		fmt.Printf("    content_hash: %s\n", chunk.ContentHash)
		fmt.Printf("    preview: %s\n", previewText(chunk.Text, 140))
	}
	fmt.Println()
	fmt.Println("OK: 文档已经按真实 tokenizer-aware chunker 完成分块。")
}

// resolveDocumentFormat 把命令行传入或文件扩展名转换成 production chunker 支持的格式。
//
// 当前已有实现支持 markdown 和 go 两类结构化解析：
// - markdown 会识别标题层级和 fenced code block；
// - go 会通过 Go AST 保留声明级结构。
func resolveDocumentFormat(source, formatName string) (documentingest.DocumentFormat, error) {
	formatName = strings.ToLower(strings.TrimSpace(formatName))
	if formatName == "" {
		switch strings.ToLower(filepath.Ext(source)) {
		case ".md", ".markdown":
			formatName = string(documentingest.FormatMarkdown)
		case ".go":
			formatName = string(documentingest.FormatGo)
		default:
			return "", fmt.Errorf("--format is required when source extension is not .md/.markdown/.go")
		}
	}
	switch documentingest.DocumentFormat(formatName) {
	case documentingest.FormatMarkdown:
		return documentingest.FormatMarkdown, nil
	case documentingest.FormatGo:
		return documentingest.FormatGo, nil
	default:
		return "", fmt.Errorf("--format must be markdown or go, got %q", formatName)
	}
}

// defaultDocumentIDFromSource 给没有显式传 --document-id 的学习命令一个稳定默认值。
//
// document_id 会进入 chunk_id 计算，所以要稳定、短、只包含安全字符。这里从文件名推导，
// 并把空格、中文或其它符号统一折叠成 '-'；正式生产系统通常会由业务侧提供逻辑 ID。
func defaultDocumentIDFromSource(source string) string {
	base := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	base = strings.TrimSpace(base)
	var out strings.Builder
	lastDash := false
	for _, char := range base {
		valid := unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_' || char == '-' || char == '.'
		if valid && char < utf8.RuneSelf {
			out.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(out.String(), "-.")
	if result == "" {
		return "document"
	}
	return result
}

// previewText 让每个 chunk 的正文可读但不刷屏。
//
// 输出里保留换行符的含义，但显示为 \n；截断按 rune 处理，避免中文被截成乱码。
func previewText(text string, limit int) string {
	text = strings.ReplaceAll(text, "\n", `\n`)
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "..."
}

// checkMySQL 检查 MySQL 是否可连接。
//
// 在这个项目的生产取向设计里，MySQL 是事实源：文档、版本、chunk manifest
// 应该能在 MySQL 里追踪。Qdrant 只是可重建的向量索引，不能反过来当事实源。
func checkMySQL(ctx context.Context, dsn string) error {
	fmt.Println("[3/6] MySQL")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open MySQL connection: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping MySQL: %w", err)
	}
	fmt.Println("  dsn: configured (redacted)")
	fmt.Println("  ping: OK")
	fmt.Println("  状态: OK")
	fmt.Println()
	return nil
}

// checkQdrant 检查 Qdrant 向量库是否可连接。
//
// Qdrant 后续负责保存 chunk vector，并在用户问题变成 query vector 后做相似度检索。
// 这里 collection 不存在只给 warning：第一步 check 不写入，真正创建/校验 collection
// 应该交给后续 ingest 步骤。
func checkQdrant(ctx context.Context, baseURL, collection string) error {
	fmt.Println("[4/6] Qdrant")
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	collection = strings.TrimSpace(collection)
	if baseURL == "" || collection == "" {
		return fmt.Errorf("Qdrant base URL and document collection are required")
	}

	var list struct {
		Result struct {
			Collections []struct {
				Name string `json:"name"`
			} `json:"collections"`
		} `json:"result"`
	}
	if err := getJSON(ctx, client, baseURL+"/collections", &list); err != nil {
		return fmt.Errorf("list Qdrant collections: %w", err)
	}

	fmt.Printf("  base_url: %s\n", baseURL)
	fmt.Printf("  目标 collection: %s\n", collection)
	fmt.Printf("  当前可见 collection 数量: %d\n", len(list.Result.Collections))

	status, detail, err := getStatus(ctx, client, baseURL+"/collections/"+url.PathEscape(collection))
	if err != nil {
		return fmt.Errorf("inspect Qdrant collection %s: %w", collection, err)
	}
	switch status {
	case http.StatusOK:
		fmt.Println("  目标 collection: 已存在")
	case http.StatusNotFound:
		fmt.Println("  目标 collection: 不存在（warning；后续 ingest 步骤可以创建）")
	default:
		return fmt.Errorf("inspect Qdrant collection %s: status %d: %s", collection, status, detail)
	}
	fmt.Println("  状态: OK")
	fmt.Println()
	return nil
}

// checkOllamaChatModel 通过 Ollama /api/show 读取 qwen:7b 元数据。
//
// 这一步还不会生成回答，只确认生成模型存在、能访问，并打印 family、架构、
// 参数规模、量化方式和 context length。后续 answer 才会真正调用 /api/chat。
func checkOllamaChatModel(baseURL, model string) error {
	fmt.Println("[5/6] Ollama 生成模型")
	client := recentchat.NewHTTPOllamaClient(baseURL)
	summary, err := client.Show(model)
	if err != nil {
		return fmt.Errorf("show Ollama chat model %s: %w", model, err)
	}
	fmt.Printf("  base_url: %s\n", strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	fmt.Printf("  model: %s\n", model)
	fmt.Printf("  family: %s\n", summary.Family)
	fmt.Printf("  架构: %s\n", summary.Architecture)
	fmt.Printf("  参数规模: %s\n", summary.ParameterSize)
	fmt.Printf("  量化方式: %s\n", summary.QuantizationLevel)
	fmt.Printf("  context_length: %d\n", summary.ContextLength)
	fmt.Println("  状态: OK")
	fmt.Println()
	return nil
}

// checkOllamaEmbedding 通过 Ollama /api/embed 检查 embedding 模型。
//
// 这是从 mock 关键词匹配走向生产向量检索的关键：文档 chunk 和用户问题都必须
// 由同一个 embedding 模型体系变成向量，Qdrant 才能比较它们的相似度。
func checkOllamaEmbedding(ctx context.Context, baseURL, model string) error {
	fmt.Println("[6/6] Ollama Embedding")
	embedder := memoryitem.NewHTTPOllamaEmbedder(baseURL)
	vectors, err := embedder.Embed(ctx, model, []string{"真实 RAG 闭环检查 embedding probe"})
	if err != nil {
		return fmt.Errorf("embed probe text with %s: %w", model, err)
	}
	if len(vectors) != 1 {
		return fmt.Errorf("embedding returned %d vectors, want 1", len(vectors))
	}
	preview := firstVectorValues(vectors[0], 5)
	fmt.Printf("  base_url: %s\n", strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	fmt.Printf("  model: %s\n", model)
	fmt.Printf("  向量维度: %d\n", len(vectors[0]))
	fmt.Printf("  前几个值: %s\n", preview)
	fmt.Println("  状态: OK")
	return nil
}

// firstVectorValues 只打印向量前几个值。
//
// 完整 embedding vector 通常有 1024 维甚至更多，单个数字也没有稳定业务含义。
// 对第一步 check 来说，更重要的是：维度正确、能返回有限数、模型调用成功。
func firstVectorValues(vector []float32, limit int) string {
	if len(vector) == 0 {
		return "[]"
	}
	if limit > len(vector) {
		limit = len(vector)
	}
	parts := make([]string, 0, limit)
	for _, value := range vector[:limit] {
		parts = append(parts, fmt.Sprintf("%.5f", value))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// getJSON 用于读取简单的 Qdrant JSON 接口。
//
// 这里没有复用更重的 Qdrant 客户端，是为了让 check 的动作一眼可见：
// GET /collections -> 解析 JSON -> 打印 collection 数量。
func getJSON(ctx context.Context, client *http.Client, endpoint string, out any) error {
	status, detail, err := getStatus(ctx, client, endpoint)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("status %d: %s", status, detail)
	}
	if strings.TrimSpace(detail) == "" {
		return errors.New("empty response body")
	}
	if err := json.Unmarshal([]byte(detail), out); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// getStatus 发一个 HTTP GET，并返回状态码和有限长度的 body。
//
// 限制 body 长度是生产习惯：外部服务错误可能很长，demo 只需要足够排障的信息，
// 不应该把终端刷爆。
func getStatus(ctx context.Context, client *http.Client, endpoint string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil {
		return resp.StatusCode, "", err
	}
	if len(body) > 4096 {
		body = append(body[:4096], []byte("...[truncated]")...)
	}
	return resp.StatusCode, strings.TrimSpace(string(body)), nil
}

// printFailureHint 根据第一处失败给出下一步怎么启动依赖。
//
// 这些提示不会掩盖真实错误；它们只是把“下一步该敲什么命令”放到错误旁边，
// 方便新人按 check -> 修复 -> 再 check 的节奏推进。
func printFailureHint(w io.Writer, err error) {
	message := err.Error()
	lower := strings.ToLower(message)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "下一步怎么继续:")
	switch {
	case strings.Contains(lower, "qdrant") && strings.Contains(lower, "connection refused"):
		fmt.Fprintln(w, "  Qdrant 没有在 127.0.0.1:6333 监听。这个项目通常用 Docker 跑 Qdrant：")
		fmt.Fprintln(w, "    docker start qdrant")
		fmt.Fprintln(w, "  如果本机还没有 qdrant 容器，可以创建一个：")
		fmt.Fprintln(w, "    docker run -d --name qdrant -p 6333:6333 -p 6334:6334 -v qdrant_storage:/qdrant/storage qdrant/qdrant:v1.18.0")
		fmt.Fprintln(w, "  启动后先确认：")
		fmt.Fprintln(w, "    curl http://127.0.0.1:6333/collections")
		fmt.Fprintln(w, "  然后重新运行本 check 命令。")
	case strings.Contains(lower, "mysql"):
		fmt.Fprintln(w, "  MySQL 无法连接。先确认 config/recent-chat.env 里的 RECENT_CHAT_MYSQL_DSN 指向本机可用库。")
		fmt.Fprintln(w, "  如果你用本机 MySQL，可以先启动 MySQL 服务；如果用 Docker，请启动对应 mysql 容器。")
		fmt.Fprintln(w, "  可用后再执行：")
		fmt.Fprintln(w, "    mysql --protocol=tcp -h 127.0.0.1 -P 3306 -u <user> -p")
	case strings.Contains(lower, "ollama") && strings.Contains(lower, "qwen"):
		fmt.Fprintln(w, "  qwen:7b 生成模型不可用。先确认 Ollama 服务和模型：")
		fmt.Fprintln(w, "    ollama serve")
		fmt.Fprintln(w, "    ollama pull qwen:7b")
		fmt.Fprintln(w, "    ollama list")
	case strings.Contains(lower, "embed") || strings.Contains(lower, "bge-m3"):
		fmt.Fprintln(w, "  embedding 模型不可用。先确认 Ollama 服务和 bge-m3：")
		fmt.Fprintln(w, "    ollama serve")
		fmt.Fprintln(w, "    ollama pull bge-m3")
		fmt.Fprintln(w, "    ollama list")
	case strings.Contains(lower, "tokenizer"):
		fmt.Fprintln(w, "  tokenizer 资产无法加载。先确认 RECENT_CHAT_TOKENIZER_PATH 指向存在的 tokenizer.json。")
		fmt.Fprintln(w, "  当前项目默认路径是：")
		fmt.Fprintln(w, "    assets/tokenizers/qwen2/tokenizer.json")
	default:
		fmt.Fprintln(w, "  先根据上面的 check failed 原因修复对应依赖，然后重新运行同一条 check 命令。")
	}
}
