package retrievalquality

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSelectPolicyIgnoresValidationTrapAndRoutesKinds(t *testing.T) {
	dataset := loadPolicyDataset(t)
	params := PolicyParams{
		DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10,
		MinimumRelevance: 0,
	}
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	outcomes := []Outcome{
		policyOutcome(SplitTrain, QueryExact, StrategySparse, params, 1, 1, 10*time.Millisecond, identity),
		policyOutcome(SplitTrain, QueryExact, StrategyDense, params, 0.8, 1, 100*time.Millisecond, identity),
		policyOutcome(SplitTrain, QueryCode, StrategyDense, params, 1, 1, 100*time.Millisecond, identity),
		policyOutcome(SplitTrain, QueryCode, StrategySparse, params, 0.9, 1, 10*time.Millisecond, identity),
		policyOutcome(SplitTrain, QuerySemantic, StrategyDense, params, 1, 1, 100*time.Millisecond, identity),
		policyOutcome(SplitTrain, QuerySemantic, StrategySparse, params, 0, 0, 10*time.Millisecond, identity),
		policyOutcome(SplitTrain, QueryMixed, StrategyHybrid, params, 1, 1, 110*time.Millisecond, identity),
		policyOutcome(SplitTrain, QueryMixed, StrategyDense, params, 0.9, 1, 100*time.Millisecond, identity),
		// If validation leaked into selection, this trap would incorrectly make Dense win exact.
		policyOutcome(SplitValidation, QueryExact, StrategyDense, params, 999, 999, time.Nanosecond, identity),
	}
	policy, err := SelectPolicy(dataset, outcomes, []PolicyParams{params})
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[QueryKind]StrategyName{
		QueryExact: StrategySparse, QueryCode: StrategyDense,
		QuerySemantic: StrategyDense, QueryMixed: StrategyHybrid,
	} {
		if got := policy.Route(kind).Strategy; got != want {
			t.Fatalf("kind=%s strategy=%s, want %s", kind, got, want)
		}
	}
	unknown := policy.Route(QueryKind("unknown"))
	if unknown.Strategy != StrategyHybrid || unknown.Params.DenseWeight != 1 ||
		unknown.Params.SparseWeight != 1 || unknown.Reason != "unknown_equal_weight_rrf" {
		t.Fatalf("unknown route=%+v", unknown)
	}
}

func TestSelectPolicyUsesStableGridTieBreakAndChecksum(t *testing.T) {
	dataset := loadPolicyDataset(t)
	left := PolicyParams{DenseWeight: 1, SparseWeight: 1, CandidateQuota: 5}
	right := PolicyParams{DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10}
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	var outcomes []Outcome
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed} {
		outcomes = append(outcomes,
			policyOutcome(SplitTrain, kind, StrategyHybrid, right, 1, 1, 10*time.Millisecond, identity),
			policyOutcome(SplitTrain, kind, StrategyHybrid, left, 1, 1, 10*time.Millisecond, identity),
		)
	}
	first, err := SelectPolicy(dataset, outcomes, []PolicyParams{right, left})
	if err != nil {
		t.Fatal(err)
	}
	rand.New(rand.NewSource(7)).Shuffle(len(outcomes), func(i, j int) {
		outcomes[i], outcomes[j] = outcomes[j], outcomes[i]
	})
	second, err := SelectPolicy(dataset, outcomes, []PolicyParams{left, right})
	if err != nil {
		t.Fatal(err)
	}
	if first.Checksum == "" || first.Checksum != second.Checksum || !reflect.DeepEqual(first.Routes, second.Routes) {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	for _, route := range first.Routes {
		if route.Params.CandidateQuota != 10 {
			t.Fatalf("tie route=%+v, want canonical JSON winner", route)
		}
	}
}

func TestSelectPolicyUsesLowerP95BeforeCanonicalTie(t *testing.T) {
	dataset := loadPolicyDataset(t)
	faster := PolicyParams{DenseWeight: 1, SparseWeight: 1, CandidateQuota: 5}
	canonical := PolicyParams{DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10}
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	var outcomes []Outcome
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed} {
		outcomes = append(outcomes,
			policyOutcome(SplitTrain, kind, StrategyHybrid, faster, 1, 1, 100*time.Millisecond, identity),
			policyOutcome(SplitTrain, kind, StrategyHybrid, canonical, 1, 1, 110*time.Millisecond, identity),
		)
	}
	policy, err := SelectPolicy(dataset, outcomes, []PolicyParams{faster, canonical})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range policy.Routes {
		if route.Params.CandidateQuota != 5 {
			t.Fatalf("lower p95 must win before canonical tie break: %+v", route)
		}
	}
}

func TestPolicyValidateIdentityRejectsDatasetOrEncoderMismatch(t *testing.T) {
	dataset := loadPolicyDataset(t)
	params := PolicyParams{DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10}
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	var outcomes []Outcome
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed} {
		outcomes = append(outcomes, policyOutcome(
			SplitTrain, kind, StrategyHybrid, params, 1, 1, time.Millisecond, identity,
		))
	}
	policy, err := SelectPolicy(dataset, outcomes, []PolicyParams{params})
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.ValidateIdentity(dataset, identity); err != nil {
		t.Fatal(err)
	}
	changed := dataset
	changed.Manifest.CasesSHA256 = "different"
	if err := policy.ValidateIdentity(changed, identity); err == nil {
		t.Fatal("changed dataset checksum should fail")
	}
	if err := policy.ValidateIdentity(dataset, EncoderIdentity{
		EmbeddingModel: "other", SparseEncoderID: identity.SparseEncoderID,
	}); err == nil {
		t.Fatal("changed encoder identity should fail")
	}
}

func TestPolicyStrategyRoutesCalibratesAndAbstains(t *testing.T) {
	policy := runtimePolicy(t, 0.25)
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "dense", Score: 0.9},
	}}}
	sparse := &recordingStrategy{name: "sparse", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "sparse", Score: 0.1},
	}}}
	strategy, err := NewPolicyStrategy(policy, dense, sparse)
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		Text: "exact query", KnowledgeScope: "scope-a", Kind: QueryExact, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Abstained || len(result.Candidates) != 0 {
		t.Fatalf("result=%+v, want calibrated threshold abstention", result)
	}
	if sparse.recordedQuery().Limit != 10 || dense.recordedQuery().Text != "" {
		t.Fatalf("sparse query=%+v dense query=%+v", sparse.recordedQuery(), dense.recordedQuery())
	}
}

func TestPolicyStrategyFallsBackInfrastructureButNotIntegrity(t *testing.T) {
	policy := runtimePolicy(t, 0)
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "dense", Score: 0.9},
	}}}
	for _, tt := range []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "infrastructure", err: &InfrastructureError{Err: errors.New("sparse down")}},
		{name: "integrity", err: &IntegrityError{Err: errors.New("scope corrupt")}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sparse := &recordingStrategy{name: "sparse", err: tt.err}
			strategy, err := NewPolicyStrategy(policy, dense, sparse)
			if err != nil {
				t.Fatal(err)
			}
			result, err := strategy.Search(context.Background(), Query{
				Text: "exact query", KnowledgeScope: "scope-a", Kind: QueryExact, Limit: 10,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("integrity failure should be hard")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Candidates) != 1 || result.Candidates[0].ChunkID != "dense" ||
				len(result.Warnings) != 2 ||
				!strings.Contains(strings.Join(result.Warnings, " "), "sparse->dense") {
				t.Fatalf("fallback result=%+v", result)
			}
		})
	}
}

func TestPolicyStrategyExecutesRouteWeights(t *testing.T) {
	policy := runtimePolicy(t, 0)
	route := policy.Routes[QueryExact]
	route.Strategy = StrategyHybrid
	route.Params.DenseWeight = 0.5
	route.Params.SparseWeight = 2
	policy.Routes[QueryExact] = route
	policy, _ = FinalizePolicy(policy)
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a"},
		{KnowledgeScope: "scope-a", ChunkID: "b"},
	}}}
	sparse := &recordingStrategy{name: "sparse", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "b"},
		{KnowledgeScope: "scope-a", ChunkID: "a"},
	}}}
	strategy, err := NewPolicyStrategy(policy, dense, sparse)
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		Text: "query", KnowledgeScope: "scope-a", Kind: QueryExact, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].ChunkID != "b" {
		t.Fatalf("weighted route was not executed: %+v", result.Candidates)
	}
}

func TestPolicyStrategyRejectsIncompleteOrInvalidArtifactContract(t *testing.T) {
	dense := &recordingStrategy{name: "dense"}
	sparse := &recordingStrategy{name: "sparse"}
	tests := []struct {
		name   string
		mutate func(*Policy)
	}{
		{
			name: "missing route",
			mutate: func(policy *Policy) {
				delete(policy.Routes, QueryCode)
			},
		},
		{
			name: "invalid strategy",
			mutate: func(policy *Policy) {
				route := policy.Routes[QueryCode]
				route.Strategy = StrategyName("other")
				policy.Routes[QueryCode] = route
			},
		},
		{
			name: "malformed calibrator",
			mutate: func(policy *Policy) {
				policy.Calibrators[StrategyDense] = Calibrator{
					Breakpoints: []float64{2, 1}, Values: []float64{0.8, 0.2},
				}
			},
		},
		{
			name: "missing route calibrator",
			mutate: func(policy *Policy) {
				delete(policy.RouteCalibrators, QueryCode)
			},
		},
		{
			name: "malformed route calibrator",
			mutate: func(policy *Policy) {
				policy.RouteCalibrators[QueryCode] = Calibrator{
					Breakpoints: []float64{1}, Values: []float64{2},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := runtimePolicy(t, 0)
			tt.mutate(&policy)
			if _, err := NewPolicyStrategy(policy, dense, sparse); err == nil {
				t.Fatal("invalid policy contract must fail")
			}
		})
	}
}

func TestPolicyStrategyCalibratesHybridDenseFallbackAsDense(t *testing.T) {
	policy := runtimePolicy(t, 0.5)
	route := policy.Routes[QueryExact]
	route.Strategy = StrategyHybrid
	policy.Routes[QueryExact] = route
	policy, _ = FinalizePolicy(policy)
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "dense", Score: 0.9},
	}}}
	sparse := &recordingStrategy{
		name: "sparse", err: &InfrastructureError{Err: errors.New("sparse down")},
	}
	strategy, err := NewPolicyStrategy(policy, dense, sparse)
	if err != nil {
		t.Fatal(err)
	}
	result, err := strategy.Search(context.Background(), Query{
		Text: "query", KnowledgeScope: "scope-a", Kind: QueryExact, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Abstained || len(result.Candidates) != 1 ||
		result.Candidates[0].Reason != "policy_dense" {
		t.Fatalf("dense fallback must use dense calibration: %+v", result)
	}
}

func TestLoadPolicyStrategyMissingArtifactUsesEqualWeightFallback(t *testing.T) {
	dataset := loadPolicyDataset(t)
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a"},
	}}}
	sparse := &recordingStrategy{name: "sparse", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "b"},
	}}}
	strategy, warnings, err := LoadPolicyStrategy(
		filepath.Join(t.TempDir(), "missing.json"), dataset, identity, dense, sparse,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strategy.Name() != "hybrid_weighted_rrf" || len(warnings) != 1 ||
		!strings.Contains(warnings[0], "equal-weight") {
		t.Fatalf("strategy=%s warnings=%v", strategy.Name(), warnings)
	}
}

func TestLoadPolicyStrategyExecutesValidArtifactAndRejectsChecksumTamper(t *testing.T) {
	dataset := loadPolicyDataset(t)
	identity := EncoderIdentity{EmbeddingModel: "bge-m3", SparseEncoderID: "encoder-v1"}
	params := PolicyParams{
		DenseWeight: 0.5, SparseWeight: 2, CandidateQuota: 10,
		MinimumRelevance: 0,
	}
	var outcomes []Outcome
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed} {
		outcomes = append(outcomes, policyOutcome(
			SplitTrain, kind, StrategyHybrid, params, 1, 1, time.Millisecond, identity,
		))
	}
	policy, err := SelectPolicy(dataset, outcomes, []PolicyParams{params})
	if err != nil {
		t.Fatal(err)
	}
	calibrator := Calibrator{Breakpoints: []float64{1}, Values: []float64{0.8}}
	policy.Calibrators = map[StrategyName]Calibrator{
		StrategyDense: calibrator, StrategySparse: calibrator, StrategyHybrid: calibrator,
	}
	policy.RouteCalibrators = map[QueryKind]Calibrator{}
	for kind := range policy.Routes {
		policy.RouteCalibrators[kind] = calibrator
	}
	policy, err = FinalizePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	dense := &recordingStrategy{name: "dense", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "a"},
		{KnowledgeScope: "scope-a", ChunkID: "b"},
	}}}
	sparse := &recordingStrategy{name: "sparse", result: SearchResult{Candidates: []Candidate{
		{KnowledgeScope: "scope-a", ChunkID: "b"},
		{KnowledgeScope: "scope-a", ChunkID: "a"},
	}}}
	strategy, warnings, err := LoadPolicyStrategy(path, dataset, identity, dense, sparse)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings=%v", warnings)
	}
	result, err := strategy.Search(context.Background(), Query{
		Text: "query", KnowledgeScope: "scope-a", Kind: QueryExact, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].ChunkID != "b" {
		t.Fatalf("artifact weights were not executed: %+v", result.Candidates)
	}

	policy.Checksum = "tampered"
	content, _ = json.Marshal(policy)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPolicyStrategy(path, dataset, identity, dense, sparse); err == nil ||
		!strings.Contains(err.Error(), "checksum") {
		t.Fatalf("tampered checksum error=%v", err)
	}
}

func policyOutcome(
	split Split,
	kind QueryKind,
	strategy StrategyName,
	params PolicyParams,
	ndcg, recall float64,
	latency time.Duration,
	identity EncoderIdentity,
) Outcome {
	return Outcome{
		Split: split, Kind: kind, Strategy: strategy, Params: params,
		NDCGAt10: ndcg, RecallAt10: recall, LatencyP95: latency,
		EncoderIdentity: identity,
	}
}

func loadPolicyDataset(t *testing.T) Dataset {
	t.Helper()
	dataset, err := LoadDataset("testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	return dataset
}

func runtimePolicy(t *testing.T, threshold float64) Policy {
	t.Helper()
	params := PolicyParams{
		DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10,
		MinimumRelevance: threshold,
	}
	policy := Policy{
		Routes: map[QueryKind]PolicyRoute{
			QueryExact:    {Strategy: StrategySparse, Params: params, Reason: "test"},
			QueryCode:     {Strategy: StrategyDense, Params: params, Reason: "test"},
			QuerySemantic: {Strategy: StrategyHybrid, Params: params, Reason: "test"},
			QueryMixed:    {Strategy: StrategyHybrid, Params: params, Reason: "test"},
		},
		UnknownRoute: PolicyRoute{Strategy: StrategyHybrid, Params: params, Reason: "unknown"},
		Calibrators: map[StrategyName]Calibrator{
			StrategyDense:  {Breakpoints: []float64{1}, Values: []float64{0.9}},
			StrategySparse: {Breakpoints: []float64{1}, Values: []float64{0.2}},
			StrategyHybrid: {Breakpoints: []float64{1}, Values: []float64{0.8}},
		},
		RouteCalibrators: map[QueryKind]Calibrator{
			QueryExact:    {Breakpoints: []float64{1}, Values: []float64{0.2}},
			QueryCode:     {Breakpoints: []float64{1}, Values: []float64{0.9}},
			QuerySemantic: {Breakpoints: []float64{1}, Values: []float64{0.8}},
			QueryMixed:    {Breakpoints: []float64{1}, Values: []float64{0.8}},
		},
	}
	finalized, err := FinalizePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	return finalized
}
