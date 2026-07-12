package memoryitem

import (
	"strings"
	"testing"
)

func TestResolveInsertsNewUpsert(t *testing.T) {
	decision, err := Resolve(nil, resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Go", 101))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionInsert || decision.Next.Version != 1 || decision.Next.Status != StatusActive || decision.Next.Value != "Go" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveReturnsNoopForEquivalentActiveValue(t *testing.T) {
	current := resolvedItem(StatusActive, "Go language", 2)
	decision, err := Resolve(&current, resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "  go   LANGUAGE ", 102))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionNoop || decision.Next.Version != 2 || decision.Next.Value != "Go language" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveUpdatesChangedValue(t *testing.T) {
	current := resolvedItem(StatusActive, "PHP", 2)
	decision, err := Resolve(&current, resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Go", 103))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionUpdate || decision.Next.Version != 3 || decision.Next.Value != "Go" || decision.Next.Status != StatusActive {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveForgetsActiveItemWithoutErasingAuditValue(t *testing.T) {
	current := resolvedItem(StatusActive, "Go", 3)
	decision, err := Resolve(&current, resolvedCandidate(OperationForget, KindProjectFact, "implementation_language", "", 104))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionForget || decision.Next.Version != 4 || decision.Next.Status != StatusForgotten || decision.Next.Value != "Go" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveRestoresForgottenItem(t *testing.T) {
	current := resolvedItem(StatusForgotten, "PHP", 4)
	decision, err := Resolve(&current, resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Go", 105))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionUpdate || decision.Next.Version != 5 || decision.Next.Status != StatusActive || decision.Next.Value != "Go" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveMissingOrForgottenForgetIsNoop(t *testing.T) {
	candidate := resolvedCandidate(OperationForget, KindProjectFact, "implementation_language", "", 106)
	missing, err := Resolve(nil, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if missing.Action != ActionNoop || missing.Next.Version != 0 {
		t.Fatalf("missing decision = %#v", missing)
	}

	current := resolvedItem(StatusForgotten, "Go", 4)
	forgotten, err := Resolve(&current, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if forgotten.Action != ActionNoop || forgotten.Next.Version != 4 {
		t.Fatalf("forgotten decision = %#v", forgotten)
	}
}

func TestResolveRejectsInvalidCurrentOrCandidateIdentity(t *testing.T) {
	validCandidate := resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Go", 101)
	tests := []struct {
		name      string
		current   Item
		candidate Candidate
		want      string
	}{
		{name: "zero ID", current: Item{UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language", Value: "PHP", Status: StatusActive, Version: 1}, candidate: validCandidate, want: "current item ID must be positive"},
		{name: "zero version", current: Item{ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language", Value: "PHP", Status: StatusActive}, candidate: validCandidate, want: "current item version must be positive"},
		{name: "invalid status", current: Item{ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language", Value: "PHP", Status: Status("pending"), Version: 1}, candidate: validCandidate, want: "unsupported current item status"},
		{name: "kind mismatch", current: resolvedItem(StatusActive, "PHP", 1), candidate: resolvedCandidate(OperationUpsert, KindPreference, "implementation_language", "Go", 101), want: "candidate identity"},
		{name: "key mismatch", current: resolvedItem(StatusActive, "PHP", 1), candidate: resolvedCandidate(OperationUpsert, KindProjectFact, "language", "Go", 101), want: "candidate identity"},
		{name: "unsupported operation", current: resolvedItem(StatusActive, "PHP", 1), candidate: resolvedCandidate(Operation("merge"), KindProjectFact, "implementation_language", "Go", 101), want: "unsupported memory operation"},
		{name: "empty upsert value", current: resolvedItem(StatusActive, "PHP", 1), candidate: resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", " ", 101), want: "upsert value is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(&tt.current, tt.candidate)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestResolveBatchUsesStableSourceOrderAndChainsSameIdentity(t *testing.T) {
	candidates := []Candidate{
		resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Go", 20),
		resolvedCandidate(OperationUpsert, KindIdentity, "name", "小黄", 10),
		resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Rust", 30),
	}

	decisions, err := ResolveBatch(nil, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 3 {
		t.Fatalf("decisions = %#v", decisions)
	}
	if decisions[0].Next.Key != "name" || decisions[0].Action != ActionInsert {
		t.Fatalf("first decision = %#v", decisions[0])
	}
	if decisions[1].Next.Key != "implementation_language" || decisions[1].Action != ActionInsert || decisions[1].Next.Version != 1 {
		t.Fatalf("second decision = %#v", decisions[1])
	}
	if decisions[2].Action != ActionUpdate || decisions[2].Next.Value != "Rust" || decisions[2].Next.Version != 2 {
		t.Fatalf("third decision = %#v", decisions[2])
	}
	if candidates[0].SourceMessageIDs[0] != 20 {
		t.Fatal("ResolveBatch mutated candidate order")
	}
}

func TestResolveBatchNormalizesIdentityBeforeChaining(t *testing.T) {
	first := resolvedCandidate(OperationUpsert, KindProjectFact, " Implementation Language ", "Go", 10)
	second := resolvedCandidate(OperationUpsert, KindProjectFact, "implementation_language", "Rust", 20)

	decisions, err := ResolveBatch(nil, []Candidate{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 || decisions[0].Action != ActionInsert || decisions[1].Action != ActionUpdate {
		t.Fatalf("decisions = %#v, want INSERT then UPDATE", decisions)
	}
	if decisions[1].Next.Version != 2 || decisions[1].Next.Key != "implementation_language" {
		t.Fatalf("second decision = %#v", decisions[1])
	}
}

func resolvedCandidate(operation Operation, kind Kind, key, value string, sourceID int64) Candidate {
	return Candidate{
		Operation: operation, Kind: kind, Key: key, Value: value,
		Confidence: 0.9, SourceMessageIDs: []int64{sourceID},
	}
}

func resolvedItem(status Status, value string, version int64) Item {
	return Item{
		ID: 7, UserID: "u-001", Kind: KindProjectFact, Key: "implementation_language",
		Value: value, Status: status, Version: version,
	}
}
