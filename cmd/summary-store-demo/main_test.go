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
