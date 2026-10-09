// qdrant-ingest-demo 是“写入一段文本”的最小可运行示例。
//
// 这节课故意不使用 MySQL。一条 Qdrant point 包含：
//
//   - vector：由 bge-m3 生成、包含 1024 个数字的语义表示；
//   - payload：可读的业务数据，例如原文和各类 ID。
//
// 以后更大的应用可能将部分业务数据保存在 MySQL。这里的目的，是先看清
// Qdrant 单独工作时是什么样子。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"offline-rag-go-lab/internal/contextretrieval"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
)

// lessonCollection 故意与项目真实使用的文档 collection 分开，避免学习实验
// 混入课程已有的数据。
const lessonCollection = "offline_rag_lesson_ingest_v1"

func main() {
	// 命令行参数让我们无需修改源码也能更换输入。配置文件提供服务地址和
	// embedding 模型名；本 demo 的文档数据并不在配置文件里。
	configPath := flag.String("config", "config/recent-chat.env", "Ollama and Qdrant config file")
	text := flag.String("text", "Qdrant 的 payload 保存原文和元数据，vector 用来做语义检索。", "text to index")
	flag.Parse()

	// 这条最小链路只读取三个配置：
	//
	//   OLLAMA_BASE_URL     embedding 模型服务监听的地址
	//   OLLAMA_EMBED_MODEL  将文本转成向量的模型名（bge-m3）
	//   QDRANT_BASE_URL     向量数据库监听的地址
	//
	// 因为本 demo 不使用 MySQL，所以没有读取任何 MySQL 配置。
	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	ollamaURL, err := fileconfig.Required(values, "OLLAMA_BASE_URL")
	if err != nil {
		log.Fatal(err)
	}
	embeddingModel, err := fileconfig.Required(values, "OLLAMA_EMBED_MODEL")
	if err != nil {
		log.Fatal(err)
	}
	qdrantURL, err := fileconfig.Required(values, "QDRANT_BASE_URL")
	if err != nil {
		log.Fatal(err)
	}

	// DocumentChunk 是应用中“一段可检索文档内容”的表达。在真实入库流程中，
	// 一篇长文档会被切成很多 chunk；本课故意只创建一个。
	//
	// 除了 Text 以外的字段会进入 Qdrant 的 payload。它们在这里不参与语义
	// 相似度计算，但使后续流程可以过滤、识别、展示和追溯返回的文本。
	chunk := contextretrieval.DocumentChunk{
		// knowledge_scope 是隔离边界：之后查询时，可以要求 Qdrant 只搜索
		// payload 中该字段恰好等于此值的 point。
		KnowledgeScope: "qdrant-lesson",
		// document_id 标识逻辑上的一篇文档；chunk_id 标识其中这一小段。
		// chunk_id 与 scope 一起决定稳定的 Qdrant point ID。
		DocumentID: "payload-introduction",
		ChunkID:    "payload-introduction-001",
		// title 和 source_ref 是人能读懂的元数据。界面或日志可以直接展示它们，
		// 不需要理解那 1024 个向量数字。
		Title:     "Payload introduction",
		SourceRef: "manual-demo",
		// Trim 防止只包含空白字符的文本被写入，也让最终保存的文本更可预期。
		Text: strings.TrimSpace(*text),
	}
	// 超时机制防止 Ollama 或 Qdrant 无法连接时，命令永久等待。defer cancel()
	// 会在 main 结束时释放相关资源。
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 第一步：把原文发送到 Ollama 的 /api/embed 接口。由于只提交了一段文本，
	// 模型会返回一个向量。它实际是含 1024 个数的 []float32，例如
	// [0.12, -0.08, ...]。这些数字用于 Qdrant 的相似度计算，逐个阅读没有
	// 业务含义。
	embedder := memoryitem.NewHTTPOllamaEmbedder(ollamaURL)
	vectors, err := embedder.Embed(ctx, embeddingModel, []string{chunk.Text})
	if err != nil {
		log.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) < 3 {
		log.Fatalf("embedding returned %d vectors; expected one vector with at least three dimensions", len(vectors))
	}
	vector := vectors[0] // 唯一的输入文本，对应返回结果中的第一个向量。

	// 第二步：连接专用的 Qdrant collection。它接近关系型数据库中的表，但需要
	// 预先确定向量维度和相似度规则。首次运行时 EnsureCollection 创建它；以后
	// 运行时则确认既有 collection 仍能接受当前的向量形状。
	store := contextretrieval.NewDocumentQdrant(qdrantURL, lessonCollection)
	if err := store.EnsureCollection(ctx, len(vector)); err != nil {
		log.Fatal(err)
	}
	// Upsert 写入一条 point。其内部向 Qdrant 发送的概念结构为：
	//
	//   {"id": stableID, "vector": [1024 numbers], "payload": {...chunk fields...}}
	//
	// “Upsert”表示：ID 不存在时插入；ID 已存在时覆盖该 point。因此重复运行
	// 本示例不会产生重复数据。
	if err := store.Upsert(ctx, chunk, vector, embeddingModel); err != nil {
		log.Fatal(err)
	}
	pointID, err := contextretrieval.DeterministicDocumentPointID(chunk.KnowledgeScope, chunk.ChunkID)
	if err != nil {
		log.Fatal(err)
	}

	// 以下输出不会再次请求数据库，而是打印刚才写入的关键数据。这样我们可以
	// 分别观察一条 point 的两个部分：面向机器的 vector 和可读的 payload。
	fmt.Printf("Collection: %s\n", lessonCollection)
	fmt.Printf("Point ID: %s\n", pointID)
	// 只展示前三个数字；把 1024 个数字全部打印出来反而会淹没本课重点。
	fmt.Printf("Vector: %d dimensions; first 3 values: %.5f, %.5f, %.5f\n", len(vector), vector[0], vector[1], vector[2])
	fmt.Println("Payload:")
	fmt.Printf("  knowledge_scope: %q\n", chunk.KnowledgeScope)
	fmt.Printf("  document_id: %q\n", chunk.DocumentID)
	fmt.Printf("  chunk_id: %q\n", chunk.ChunkID)
	fmt.Printf("  title: %q\n", chunk.Title)
	fmt.Printf("  text: %q\n", chunk.Text)
	fmt.Println("Upsert complete: re-running this command updates this same point ID.")
}
