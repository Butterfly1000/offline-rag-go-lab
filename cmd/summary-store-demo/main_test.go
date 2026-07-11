package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

type fakeSchemaExecutor struct {
	query string
}

func (e *fakeSchemaExecutor) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	e.query = query
	return fakeCommandResult(1), nil
}

type fakeCommandResult int64

func (r fakeCommandResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeCommandResult) RowsAffected() (int64, error) { return int64(r), nil }

func TestReadConfigValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-chat.env")
	content := "# local config\nRECENT_CHAT_MYSQL_DSN = user:pass@tcp(127.0.0.1:3306)/offline_rag?parseTime=true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readConfigValue(path, "RECENT_CHAT_MYSQL_DSN")
	if err != nil {
		t.Fatalf("readConfigValue() error = %v", err)
	}
	if got != "user:pass@tcp(127.0.0.1:3306)/offline_rag?parseTime=true" {
		t.Fatalf("readConfigValue() = %q", got)
	}
}

func TestReadConfigValueRejectsMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-chat.env")
	if err := os.WriteFile(path, []byte("OTHER=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigValue(path, "RECENT_CHAT_MYSQL_DSN"); err == nil {
		t.Fatal("readConfigValue() error = nil, want missing key error")
	}
}

func TestExecuteSchemaReadsAndExecutesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.sql")
	if err := os.WriteFile(path, []byte("CREATE TABLE demo (id BIGINT);"), 0o600); err != nil {
		t.Fatal(err)
	}
	executor := &fakeSchemaExecutor{}
	if err := executeSchema(context.Background(), executor, path); err != nil {
		t.Fatalf("executeSchema() error = %v", err)
	}
	if executor.query != "CREATE TABLE demo (id BIGINT);" {
		t.Fatalf("executeSchema() query = %q", executor.query)
	}
}
