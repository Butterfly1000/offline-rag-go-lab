package fileconfig

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Load parses the project's simple KEY=VALUE config format without exporting
// values into the process environment. Blank lines and full-line comments are ignored.
func Load(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("config %s line %d must use KEY=VALUE", path, lineNumber)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("config %s repeats key %s", path, key)
		}
		values[key] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan config %s: %w", path, err)
	}
	return values, nil
}

func Required(values map[string]string, key string) (string, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return "", fmt.Errorf("config value %s is required", key)
	}
	return value, nil
}

// IntOrDefault keeps numeric service settings in the same local file as the DSN.
func IntOrDefault(values map[string]string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(values[key])
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse config value %s=%q as integer: %w", key, raw, err)
	}
	return value, nil
}
