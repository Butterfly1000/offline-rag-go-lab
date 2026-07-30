package retrievalquality

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestFuseRRFUsesRanksAndRecordsContributions(t *testing.T) {
	dense := []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a", Score: 0.99},
		{KnowledgeScope: "scope-a", ChunkID: "b", Score: 0.50},
	}
	sparse := []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "b", Score: 1000},
		{KnowledgeScope: "scope-a", ChunkID: "a", Score: 1},
	}
	fused, err := FuseRRF(dense, sparse, FusionWeights{Dense: 1, Sparse: 1}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{fused[0].ChunkID, fused[1].ChunkID}; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("order=%v, want stable chunk ID tie-break", got)
	}
	want := 1.0/61.0 + 1.0/62.0
	if math.Abs(fused[0].Score-want) > 1e-12 {
		t.Fatalf("score=%v, want %v", fused[0].Score, want)
	}
	if fused[0].Trace["dense_rank"] != 1 || fused[0].Trace["sparse_rank"] != 2 {
		t.Fatalf("trace=%v", fused[0].Trace)
	}
	if fused[0].Reason != "hybrid_rrf" {
		t.Fatalf("reason=%q", fused[0].Reason)
	}
}

func TestFuseRRFDoesNotUseRawScoreScaleForOrdering(t *testing.T) {
	left, err := FuseRRF(
		[]Candidate{{KnowledgeScope: "s", ChunkID: "a", Score: 0.1}, {KnowledgeScope: "s", ChunkID: "b", Score: 0.9}},
		[]Candidate{{KnowledgeScope: "s", ChunkID: "b", Score: 1}, {KnowledgeScope: "s", ChunkID: "a", Score: 1000000}},
		FusionWeights{Dense: 2, Sparse: 1}, 60,
	)
	if err != nil {
		t.Fatal(err)
	}
	right, err := FuseRRF(
		[]Candidate{{KnowledgeScope: "s", ChunkID: "a", Score: 999999}, {KnowledgeScope: "s", ChunkID: "b", Score: -50}},
		[]Candidate{{KnowledgeScope: "s", ChunkID: "b", Score: -100}, {KnowledgeScope: "s", ChunkID: "a", Score: 0}},
		FusionWeights{Dense: 2, Sparse: 1}, 60,
	)
	if err != nil {
		t.Fatal(err)
	}
	if left[0].ChunkID != right[0].ChunkID || left[0].Score != right[0].Score {
		t.Fatalf("raw score changed fused order: left=%+v right=%+v", left, right)
	}
}

func TestFuseRRFRejectsDuplicateAndConflictingCandidates(t *testing.T) {
	tests := []struct {
		name   string
		dense  []Candidate
		sparse []Candidate
		want   string
	}{
		{
			name: "duplicate",
			dense: []Candidate{
				{KnowledgeScope: "s", ChunkID: "a"},
				{KnowledgeScope: "s", ChunkID: "a"},
			},
			want: "duplicate",
		},
		{
			name:   "scope conflict",
			dense:  []Candidate{{KnowledgeScope: "s1", ChunkID: "a"}},
			sparse: []Candidate{{KnowledgeScope: "s2", ChunkID: "a"}},
			want:   "scope",
		},
		{
			name:  "empty identity",
			dense: []Candidate{{KnowledgeScope: "s", ChunkID: ""}},
			want:  "identity",
		},
		{
			name: "mixed scopes in one leg",
			dense: []Candidate{
				{KnowledgeScope: "s1", ChunkID: "a"},
				{KnowledgeScope: "s2", ChunkID: "b"},
			},
			want: "scope",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FuseRRF(tt.dense, tt.sparse, FusionWeights{Dense: 1, Sparse: 1}, 60)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %q", err, tt.want)
			}
		})
	}
}
