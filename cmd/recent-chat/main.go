package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/chatprompt"
	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/promptbudget"
	"offline-rag-go-lab/internal/recentchat"
	"offline-rag-go-lab/internal/sessionsummary"
	"offline-rag-go-lab/internal/tokenizerdemo"
)

func main() {
	configPath := flag.String("config", "config/recent-chat.env", "local service config file")
	addr := flag.String("addr", ":18093", "HTTP listen address")
	summaryMinMessagesFlag := flag.Int("summary-min-messages", 0, "override summary message threshold")
	summaryMinTokensFlag := flag.Int("summary-min-tokens", 0, "override summary token threshold")
	summaryInputReserveFlag := flag.Int("summary-input-reserve", 0, "override summary input reserve")
	summaryOutputLimitFlag := flag.Int("summary-output-limit", 0, "override summary generation limit")
	flag.Parse()

	values, err := fileconfig.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	dsn, err := fileconfig.Required(values, "RECENT_CHAT_MYSQL_DSN")
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tokenizerPath := valueOrDefault(values, "RECENT_CHAT_TOKENIZER_PATH", filepath.Join("assets", "tokenizers", "qwen2", "tokenizer.json"))
	tokenCounter, err := tokenizerdemo.LoadCounter(tokenizerPath)
	if err != nil {
		log.Fatal(err)
	}

	formatter := chatprompt.QwenFormatter{}
	ollama := recentchat.NewHTTPOllamaClient(valueOrDefault(values, "OLLAMA_BASE_URL", "http://127.0.0.1:11434"))
	automaticBudget := promptbudget.NewAutomaticPlanner(
		ollama,
		chatprompt.NewTokenCounter(tokenCounter, formatter),
	)
	summaryMinMessages := configInt(values, "SESSION_SUMMARY_MIN_MESSAGES", 8, *summaryMinMessagesFlag)
	summaryMinTokens := configInt(values, "SESSION_SUMMARY_MIN_TOKENS", 2048, *summaryMinTokensFlag)
	summaryInputReserve := configInt(values, "SESSION_SUMMARY_INPUT_RESERVE", 1024, *summaryInputReserveFlag)
	summaryOutputLimit := configInt(values, "SESSION_SUMMARY_OUTPUT_LIMIT", 512, *summaryOutputLimitFlag)
	if summaryInputReserve <= 0 || summaryOutputLimit <= 0 || summaryOutputLimit >= summaryInputReserve {
		log.Fatalf("summary limits require 0 < output (%d) < input reserve (%d)", summaryOutputLimit, summaryInputReserve)
	}
	summaryPolicy, err := sessionsummary.NewTriggerPolicy(summaryMinMessages, summaryMinTokens)
	if err != nil {
		log.Fatal(err)
	}
	messageStore := recentchat.NewMySQLMessageStore(db)
	summaryStore := sessionsummary.NewMySQLSummaryStore(db)
	summaryUpdater := sessionsummary.NewUpdateService(
		summaryStore,
		sessionsummary.NewMySQLMessageSource(db),
		sessionsummary.NewFormattedMessageCounter(tokenCounter, formatter),
		summaryPolicy,
		sessionsummary.NewGenerator(ollama),
	)
	service := recentchat.NewServiceWithSessionSummary(
		messageStore,
		recentchat.CountWindowBuilder{},
		recentchat.NewFormattedTokenBudgetWindowBuilder(tokenCounter, formatter),
		ollama,
		automaticBudget,
		summaryUpdater,
		summaryStore,
		summaryInputReserve,
		summaryOutputLimit,
	)

	mux := http.NewServeMux()
	recentchat.RegisterHandlers(mux, service)

	log.Printf("recent-chat listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func valueOrDefault(values map[string]string, name, fallback string) string {
	if value := values[name]; value != "" {
		return value
	}
	return fallback
}

func configInt(values map[string]string, name string, fallback, override int) int {
	value, err := fileconfig.IntOrDefault(values, name, fallback)
	if err != nil {
		log.Fatal(err)
	}
	if override > 0 {
		return override
	}
	return value
}
