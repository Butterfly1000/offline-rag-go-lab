package fileconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsKeyValuesAndIgnoresComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.env")
	content := "# local values\nMYSQL_DSN = user:pass@tcp(localhost:3306)/db\nOLLAMA_URL=http://127.0.0.1:11434\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if values["MYSQL_DSN"] != "user:pass@tcp(localhost:3306)/db" || values["OLLAMA_URL"] != "http://127.0.0.1:11434" {
		t.Fatalf("Load() = %#v", values)
	}
}

func TestRequiredRejectsMissingAndEmptyValues(t *testing.T) {
	if _, err := Required(map[string]string{}, "MYSQL_DSN"); err == nil {
		t.Fatal("Required() error=nil, want missing value error")
	}
	if _, err := Required(map[string]string{"MYSQL_DSN": "  "}, "MYSQL_DSN"); err == nil {
		t.Fatal("Required() error=nil, want empty value error")
	}
}

func TestIntOrDefaultParsesValueAndUsesFallback(t *testing.T) {
	values := map[string]string{"LIMIT": "256"}
	if got, err := IntOrDefault(values, "LIMIT", 128); err != nil || got != 256 {
		t.Fatalf("IntOrDefault()=(%d,%v), want (256,nil)", got, err)
	}
	if got, err := IntOrDefault(values, "MISSING", 128); err != nil || got != 128 {
		t.Fatalf("IntOrDefault()=(%d,%v), want fallback", got, err)
	}
	if _, err := IntOrDefault(map[string]string{"LIMIT": "bad"}, "LIMIT", 128); err == nil {
		t.Fatal("IntOrDefault() error=nil, want parse error")
	}
}
