package main

import (
	"bufio"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"offline-rag-go-lab/internal/recentchat"
)

func main() {
	loadEnvFile("config/recent-chat.env")

	dsn := os.Getenv("RECENT_CHAT_MYSQL_DSN")
	if dsn == "" {
		log.Fatal("RECENT_CHAT_MYSQL_DSN is required")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	service := recentchat.NewService(
		recentchat.NewMySQLMessageStore(db),
		recentchat.CountWindowBuilder{},
		recentchat.NewHTTPOllamaClient(envOrDefault("OLLAMA_BASE_URL", "http://127.0.0.1:11434")),
	)

	mux := http.NewServeMux()
	recentchat.RegisterHandlers(mux, service)

	addr := ":18093"
	log.Printf("recent-chat listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			log.Fatalf("set env %s from %s: %v", key, path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scan %s: %v", path, err)
	}
}
