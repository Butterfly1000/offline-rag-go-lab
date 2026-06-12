// Package boss_test 用外部测试包验证整条 gateway 编排链路（黑盒风格）。
package boss_test

import (
	"os"         // ReadDir / ReadFile 检查日志文件
	"path/filepath" // 临时目录路径拼接
	"strings"    // Contains 断言回答内容
	"testing"

	gateway "offline-rag-go-lab/internal/gateway"
)

// newTestApp 为每个测试创建独立临时目录，避免污染 storage/ 和测试间互相影响。
func newTestApp(t *testing.T) *gateway.App {
	t.Helper() // 失败时堆栈显示调用方测试函数，而非 newTestApp

	baseDir := t.TempDir() // 测试结束自动删除的临时目录
	app := gateway.NewApp(gateway.Config{
		LogDir:              filepath.Join(baseDir, "logs"),
		DocDir:              filepath.Join(baseDir, "docs"),
		RetrievalTopK:       3,
		ScoreThreshold:      0.1,
		PromptMaxChunks:     3,
		PromptMaxChars:      300,
		ChatModel:           "mock-chat",
		EmbeddingModel:      "mock-embedding",
		KnowledgeCollection: "knowledge_chunks",
	})
	return app
}

// TestIngestThenDebugRetrievalReturnsExpectedChunk ingest 后检索应命中对应文档。
func TestIngestThenDebugRetrievalReturnsExpectedChunk(t *testing.T) {
	app := newTestApp(t)

	_, err := app.IngestText(gateway.IngestRequest{
		DocumentID: "refund-policy",
		Title:      "退款政策",
		SourceRef:  "refund-policy.md",
		Text:       "用户在购买后 7 天内可以申请退款。\n处理流程包括提交订单号、说明退款原因、等待人工审核。",
		Tags:       []string{"faq", "refund"},
	})
	if err != nil {
		t.Fatalf("IngestText returned error: %v", err)
	}

	result := app.DebugRetrieval("退款需要什么步骤")
	if len(result.Hits) == 0 {
		t.Fatalf("expected at least one retrieval hit")
	}
	if result.Hits[0].DocumentID != "refund-policy" {
		t.Fatalf("expected document_id refund-policy, got %s", result.Hits[0].DocumentID)
	}
	if !strings.Contains(result.Hits[0].Preview, "退款") {
		t.Fatalf("expected preview to include 退款, got %s", result.Hits[0].Preview)
	}
}

// TestSplitPreviewReturnsParagraphChunks 空行分段应得到 3 个 chunk，ID 为 guide#0,#1,#2。
func TestSplitPreviewReturnsParagraphChunks(t *testing.T) {
	app := newTestApp(t)

	resp, err := app.SplitPreview(gateway.IngestRequest{
		DocumentID: "guide",
		Title:      "使用说明",
		SourceRef:  "guide.md",
		Text:       "第一段介绍。\n\n第二段讲步骤。\n第三段讲注意事项。",
		Tags:       []string{"guide"},
	})
	if err != nil {
		t.Fatalf("SplitPreview returned error: %v", err)
	}

	if len(resp.Chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(resp.Chunks))
	}
	if resp.Chunks[0].ChunkID != "guide#0" {
		t.Fatalf("expected first chunk id guide#0, got %s", resp.Chunks[0].ChunkID)
	}
	if !strings.Contains(resp.Chunks[1].Text, "第二段") {
		t.Fatalf("expected second chunk text to contain 第二段, got %s", resp.Chunks[1].Text)
	}
}

// TestChatUsesKnowledgeWhenRelevantChunkExists 有相关文档时 Chat 应 UsedKnowledge=true 且回答含关键词。
func TestChatUsesKnowledgeWhenRelevantChunkExists(t *testing.T) {
	app := newTestApp(t)

	_, err := app.IngestText(gateway.IngestRequest{
		DocumentID: "faq",
		Title:      "常见问题",
		SourceRef:  "faq.md",
		Text:       "系统支持 PDF、Markdown、TXT 文档上传。\n知识库问答会先检索相关内容，再生成答案。",
		Tags:       []string{"faq"},
	})
	if err != nil {
		t.Fatalf("IngestText returned error: %v", err)
	}

	resp, err := app.Chat(gateway.ChatRequest{
		SessionID:    "s-001",
		UserID:       "u-001",
		Question:     "支持上传哪些文档格式？",
		Model:        "mock-chat",
		UseKnowledge: true,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if !resp.UsedKnowledge {
		t.Fatalf("expected UsedKnowledge to be true")
	}
	if len(resp.RetrievedChunks) == 0 {
		t.Fatalf("expected retrieval chunks to be returned")
	}
	if !strings.Contains(resp.Answer, "PDF") {
		t.Fatalf("expected answer to include PDF, got %s", resp.Answer)
	}
}

// TestPromptPreviewIncludesRetrievedKnowledgeSection prompt 预览应含 [Relevant Knowledge] 和命中正文。
func TestPromptPreviewIncludesRetrievedKnowledgeSection(t *testing.T) {
	app := newTestApp(t)

	_, err := app.IngestText(gateway.IngestRequest{
		DocumentID: "faq",
		Title:      "常见问题",
		SourceRef:  "faq.md",
		Text:       "系统支持 PDF、Markdown、TXT 文档上传。\n知识库问答会先检索相关内容，再生成答案。",
		Tags:       []string{"faq"},
	})
	if err != nil {
		t.Fatalf("IngestText returned error: %v", err)
	}

	resp := app.PromptPreview("支持上传哪些文档格式？")
	if len(resp.SelectedChunks) == 0 {
		t.Fatalf("expected prompt preview to select chunks")
	}
	if !strings.Contains(resp.Prompt, "[Relevant Knowledge]") {
		t.Fatalf("expected prompt preview to include knowledge section, got %s", resp.Prompt)
	}
	if !strings.Contains(resp.Prompt, "PDF") {
		t.Fatalf("expected prompt preview to include retrieved text, got %s", resp.Prompt)
	}
}

// TestChatFallsBackGracefullyWhenNoKnowledgeHit 无关问题应不命中知识库，走 mock 兜底话术。
func TestChatFallsBackGracefullyWhenNoKnowledgeHit(t *testing.T) {
	app := newTestApp(t)

	resp, err := app.Chat(gateway.ChatRequest{
		SessionID:    "s-002",
		UserID:       "u-002",
		Question:     "火星上的天气怎么样？",
		Model:        "mock-chat",
		UseKnowledge: true,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	if resp.UsedKnowledge {
		t.Fatalf("expected UsedKnowledge to be false")
	}
	if len(resp.RetrievedChunks) != 0 {
		t.Fatalf("expected no retrieved chunks, got %d", len(resp.RetrievedChunks))
	}
	if !strings.Contains(resp.Answer, "未命中知识库") {
		t.Fatalf("expected fallback answer, got %s", resp.Answer)
	}
}

// TestChatWritesJSONLLog Chat 成功后应在 LogDir 生成含 session_id 的 JSONL 行。
func TestChatWritesJSONLLog(t *testing.T) {
	baseDir := t.TempDir()
	app := gateway.NewApp(gateway.Config{
		LogDir:              filepath.Join(baseDir, "logs"),
		DocDir:              filepath.Join(baseDir, "docs"),
		RetrievalTopK:       3,
		ScoreThreshold:      0.1,
		PromptMaxChunks:     3,
		PromptMaxChars:      300,
		ChatModel:           "mock-chat",
		EmbeddingModel:      "mock-embedding",
		KnowledgeCollection: "knowledge_chunks",
	})

	_, err := app.IngestText(gateway.IngestRequest{
		DocumentID: "intro",
		Title:      "产品介绍",
		SourceRef:  "intro.md",
		Text:       "这是一个离线知识问答系统。",
		Tags:       []string{"intro"},
	})
	if err != nil {
		t.Fatalf("IngestText returned error: %v", err)
	}

	_, err = app.Chat(gateway.ChatRequest{
		SessionID:    "s-003",
		UserID:       "u-003",
		Question:     "这是什么系统？",
		Model:        "mock-chat",
		UseKnowledge: true,
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	files, err := os.ReadDir(filepath.Join(baseDir, "logs"))
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one log file, got %d", len(files))
	}
	raw, err := os.ReadFile(filepath.Join(baseDir, "logs", files[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(raw), `"session_id":"s-003"`) {
		t.Fatalf("expected log to include session id, got %s", string(raw))
	}
}
