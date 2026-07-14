package main

import (
	"testing"

	"offline-rag-go-lab/internal/memoryitem"
)

func TestMemoryEmbeddingTextIncludesStableIdentityAndValue(t *testing.T) {
	item := memoryitem.Item{
		ID: 7, UserID: "u-001", Kind: memoryitem.KindProjectFact,
		Key: "implementation_language", Value: "Go", Status: memoryitem.StatusActive, Version: 3,
	}
	if got := memoryEmbeddingText(item); got != "project_fact/implementation_language: Go" {
		t.Fatalf("memoryEmbeddingText() = %q", got)
	}
}

func TestValidateTopResultEnforcesUserAndExpectedItem(t *testing.T) {
	valid := []memoryitem.SearchResult{{ItemID: 7, UserID: "u-001", Kind: memoryitem.KindProjectFact, Key: "implementation_language", Value: "Go"}}
	if err := validateTopResult(valid, "u-001", 7); err != nil {
		t.Fatal(err)
	}
	if err := validateTopResult(valid, "u-002", 7); err == nil {
		t.Fatal("cross-user result error = nil")
	}
	if err := validateTopResult(nil, "u-001", 7); err == nil {
		t.Fatal("empty result error = nil")
	}
}

func TestContainsItemDetectsForgottenOrCrossUserPoint(t *testing.T) {
	results := []memoryitem.SearchResult{{ItemID: 7, UserID: "u-001"}, {ItemID: 8, UserID: "u-002"}}
	if !containsItem(results, 7) || containsItem(results, 9) {
		t.Fatalf("containsItem() results = %#v", results)
	}
	if !containsOtherUser(results, "u-001") || containsOtherUser(results[:1], "u-001") {
		t.Fatalf("containsOtherUser() results = %#v", results)
	}
}

func TestValidateDemoCollectionAllowsOnlyDedicatedCollection(t *testing.T) {
	if err := validateDemoCollection(demoCollection); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"", "ollama_chat_memory", "another_existing_collection"} {
		if err := validateDemoCollection(collection); err == nil {
			t.Fatalf("validateDemoCollection(%q) error = nil", collection)
		}
	}
}
