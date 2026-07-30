package main

import "testing"

func TestValidateLocalHTTPURL(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1:6333",
		"http://localhost:11434",
		"http://[::1]:6333",
	} {
		if err := validateLocalHTTPURL("service", raw); err != nil {
			t.Fatalf("local URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"https://qdrant.example.com",
		"http://192.168.1.20:6333",
		"file:///tmp/socket",
		"not-a-url",
	} {
		if err := validateLocalHTTPURL("service", raw); err == nil {
			t.Fatalf("non-local URL %q must fail", raw)
		}
	}
}
