package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"offline-rag-go-lab/internal/contextretrieval"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local tokenizer config file")
	contextBudget := flag.Int("context-token-budget", 160, "maximum tokens for rendered retrieved context")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	tokenizerPath, err := fileconfig.Required(values, "RECENT_CHAT_TOKENIZER_PATH")
	if err != nil {
		log.Fatal(err)
	}
	counter, err := tokenizerdemo.LoadCounter(tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}

	memory, documents := demoCandidates()
	merged, err := contextretrieval.Merge(memory, documents, contextretrieval.MergeLimits{
		Memory: 1, Documents: len(documents),
	})
	if err != nil {
		log.Fatal(err)
	}
	selection, err := contextretrieval.SelectWithinTokenBudget(merged, *contextBudget, counter)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Memory candidates: %d\n", len(memory))
	fmt.Printf("Document candidates: %d\n", len(documents))
	fmt.Printf("Duplicate removed: %t\n", !containsID(merged, "document:duplicate"))
	fmt.Printf("Merged source order: %s\n", sourceOrder(merged))
	fmt.Printf("Selected source order: %s\n", sourceOrder(selection.Hits))
	fmt.Printf("Dropped IDs: %s\n", strings.Join(selection.DroppedIDs, ","))
	fmt.Printf("Used context tokens: %d\n", selection.UsedTokens)
	fmt.Printf("Within budget: %t\n", selection.UsedTokens <= *contextBudget)
	fmt.Printf("Rendered retrieved_context:\n%s\n", selection.Rendered)
}

func demoCandidates() ([]contextretrieval.Hit, []contextretrieval.Hit) {
	memory := []contextretrieval.Hit{
		{
			Source: contextretrieval.SourceMemory, ID: "memory:language",
			Content: "这个项目使用 Go。", Score: 0.92, UserID: "u-001", Kind: "project_fact",
		},
		{
			Source: contextretrieval.SourceMemory, ID: "memory:preference",
			Content: "教学优先展示真实实践。", Score: 0.81, UserID: "u-001", Kind: "preference",
		},
	}
	documents := []contextretrieval.Hit{
		{
			Source: contextretrieval.SourceDocument, ID: "document:duplicate",
			Content: "  这个项目使用   Go。 ", Score: 0.99, KnowledgeScope: "offline-rag-course",
			Title: "重复事实", SourceRef: "fixtures/duplicate.md",
		},
		{
			Source: contextretrieval.SourceDocument, ID: "document:oversized",
			Content: strings.Repeat("这是一段超出上下文预算的完整文档内容。", 80), Score: 0.95,
			KnowledgeScope: "offline-rag-course", Title: "超长文档", SourceRef: "fixtures/oversized.md",
		},
		{
			Source: contextretrieval.SourceDocument, ID: "document:token-budget",
			Content: "用 tokenizer 计算完整上下文。", Score: 0.90,
			KnowledgeScope: "offline-rag-course", Title: "Token", SourceRef: "token.md",
		},
		{
			Source: contextretrieval.SourceDocument, ID: "document:recent-window",
			Content: "按顺序读取最近消息。", Score: 0.80,
			KnowledgeScope: "offline-rag-course", Title: "Recent", SourceRef: "recent.md",
		},
	}
	return memory, documents
}

func sourceOrder(hits []contextretrieval.Hit) string {
	sources := make([]string, len(hits))
	for index, hit := range hits {
		sources[index] = string(hit.Source)
	}
	return strings.Join(sources, ",")
}

func containsID(hits []contextretrieval.Hit, id string) bool {
	for _, hit := range hits {
		if hit.ID == id {
			return true
		}
	}
	return false
}
