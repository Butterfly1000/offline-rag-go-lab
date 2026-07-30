package retrievalquality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type StrategyName string

const (
	StrategyDense  StrategyName = "dense"
	StrategySparse StrategyName = "sparse"
	StrategyHybrid StrategyName = "hybrid"
)

type PolicyParams struct {
	DenseWeight      float64 `json:"dense_weight"`
	SparseWeight     float64 `json:"sparse_weight"`
	CandidateQuota   int     `json:"candidate_quota"`
	MinimumRelevance float64 `json:"minimum_relevance"`
}

type EncoderIdentity struct {
	EmbeddingModel  string `json:"embedding_model"`
	SparseEncoderID string `json:"sparse_encoder_id"`
}

type Outcome struct {
	Split           Split           `json:"split"`
	Kind            QueryKind       `json:"kind"`
	Strategy        StrategyName    `json:"strategy"`
	Params          PolicyParams    `json:"params"`
	NDCGAt10        float64         `json:"ndcg_at_10"`
	RecallAt10      float64         `json:"recall_at_10"`
	LatencyP95      time.Duration   `json:"latency_p95_ns"`
	EncoderIdentity EncoderIdentity `json:"encoder_identity"`
}

type PolicyRoute struct {
	Strategy StrategyName `json:"strategy"`
	Params   PolicyParams `json:"params"`
	Reason   string       `json:"reason"`
}

type Policy struct {
	DatasetID        string                      `json:"dataset_id"`
	DatasetVersion   string                      `json:"dataset_version"`
	CorpusSHA256     string                      `json:"corpus_sha256"`
	CasesSHA256      string                      `json:"cases_sha256"`
	EncoderIdentity  EncoderIdentity             `json:"encoder_identity"`
	Routes           map[QueryKind]PolicyRoute   `json:"routes"`
	UnknownRoute     PolicyRoute                 `json:"unknown_route"`
	Calibrators      map[StrategyName]Calibrator `json:"calibrators"`
	RouteCalibrators map[QueryKind]Calibrator    `json:"route_calibrators"`
	Checksum         string                      `json:"policy_checksum"`
}

func DefaultPolicyGrid() []PolicyParams {
	weights := []float64{0.5, 1, 2}
	quotas := []int{5, 10, 20}
	thresholds := []float64{0, 0.25, 0.5}
	result := make([]PolicyParams, 0, len(weights)*len(weights)*len(quotas)*len(thresholds))
	for _, denseWeight := range weights {
		for _, sparseWeight := range weights {
			for _, quota := range quotas {
				for _, threshold := range thresholds {
					result = append(result, PolicyParams{
						DenseWeight: denseWeight, SparseWeight: sparseWeight,
						CandidateQuota: quota, MinimumRelevance: threshold,
					})
				}
			}
		}
	}
	return result
}

func SelectPolicy(dataset Dataset, outcomes []Outcome, grid []PolicyParams) (Policy, error) {
	if len(grid) == 0 {
		return Policy{}, fmt.Errorf("policy parameter grid is required")
	}
	gridKeys := make(map[string]bool, len(grid))
	for index, params := range grid {
		if err := validatePolicyParams(params); err != nil {
			return Policy{}, fmt.Errorf("policy grid item %d: %w", index, err)
		}
		key, err := canonicalParams(params)
		if err != nil {
			return Policy{}, err
		}
		gridKeys[key] = true
	}
	var train []Outcome
	var identity EncoderIdentity
	for index, outcome := range outcomes {
		if outcome.Split == SplitValidation {
			continue
		}
		if outcome.Split != SplitTrain {
			return Policy{}, fmt.Errorf("policy outcome %d has invalid split %q", index, outcome.Split)
		}
		if !selectableKind(outcome.Kind) {
			continue
		}
		if outcome.Strategy != StrategyDense && outcome.Strategy != StrategySparse && outcome.Strategy != StrategyHybrid {
			return Policy{}, fmt.Errorf("policy outcome %d has invalid strategy %q", index, outcome.Strategy)
		}
		if !finite(outcome.NDCGAt10) || !finite(outcome.RecallAt10) ||
			outcome.NDCGAt10 < 0 || outcome.NDCGAt10 > 1 ||
			outcome.RecallAt10 < 0 || outcome.RecallAt10 > 1 || outcome.LatencyP95 < 0 {
			return Policy{}, fmt.Errorf("policy outcome %d has invalid metrics", index)
		}
		key, err := canonicalParams(outcome.Params)
		if err != nil {
			return Policy{}, fmt.Errorf("policy outcome %d: %w", index, err)
		}
		if !gridKeys[key] {
			continue
		}
		currentIdentity := normalizeEncoderIdentity(outcome.EncoderIdentity)
		if currentIdentity.EmbeddingModel == "" || currentIdentity.SparseEncoderID == "" {
			return Policy{}, fmt.Errorf("policy outcome %d encoder identity is required", index)
		}
		if identity.EmbeddingModel == "" {
			identity = currentIdentity
		} else if identity != currentIdentity {
			return Policy{}, fmt.Errorf("policy train outcomes mix encoder identities")
		}
		train = append(train, outcome)
	}
	if len(train) == 0 {
		return Policy{}, fmt.Errorf("matching train outcomes are required")
	}
	routes := make(map[QueryKind]PolicyRoute, 4)
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed} {
		var candidates []Outcome
		for _, outcome := range train {
			if outcome.Kind == kind {
				candidates = append(candidates, outcome)
			}
		}
		if len(candidates) == 0 {
			return Policy{}, fmt.Errorf("train outcomes for query kind %q are required", kind)
		}
		sort.Slice(candidates, func(i, j int) bool {
			return betterOutcome(candidates[i], candidates[j])
		})
		winner := candidates[0]
		routes[kind] = PolicyRoute{
			Strategy: winner.Strategy, Params: winner.Params,
			Reason: "selected_on_train_ndcg_recall_latency_canonical_params",
		}
	}
	unknown := PolicyRoute{
		Strategy: StrategyHybrid,
		Params: PolicyParams{
			DenseWeight: 1, SparseWeight: 1, CandidateQuota: 10,
			MinimumRelevance: 0,
		},
		Reason: "unknown_equal_weight_rrf",
	}
	policy := Policy{
		DatasetID: dataset.Manifest.DatasetID, DatasetVersion: dataset.Manifest.Version,
		CorpusSHA256: dataset.Manifest.CorpusSHA256, CasesSHA256: dataset.Manifest.CasesSHA256,
		EncoderIdentity: identity, Routes: routes, UnknownRoute: unknown,
		Calibrators: map[StrategyName]Calibrator{}, RouteCalibrators: map[QueryKind]Calibrator{},
	}
	return FinalizePolicy(policy)
}

func (p Policy) Route(kind QueryKind) PolicyRoute {
	if route, exists := p.Routes[kind]; exists {
		return route
	}
	return p.UnknownRoute
}

func (p Policy) ValidateIdentity(dataset Dataset, identity EncoderIdentity) error {
	if p.DatasetID != dataset.Manifest.DatasetID || p.DatasetVersion != dataset.Manifest.Version ||
		p.CorpusSHA256 != dataset.Manifest.CorpusSHA256 || p.CasesSHA256 != dataset.Manifest.CasesSHA256 {
		return fmt.Errorf("policy dataset identity mismatch")
	}
	if p.EncoderIdentity != normalizeEncoderIdentity(identity) {
		return fmt.Errorf("policy encoder identity mismatch")
	}
	finalized, err := FinalizePolicy(p)
	if err != nil {
		return err
	}
	if finalized.Checksum != p.Checksum {
		return fmt.Errorf("policy checksum mismatch")
	}
	return nil
}

func FinalizePolicy(policy Policy) (Policy, error) {
	policy.Checksum = ""
	content, err := json.Marshal(policy)
	if err != nil {
		return Policy{}, fmt.Errorf("encode policy checksum: %w", err)
	}
	sum := sha256.Sum256(content)
	policy.Checksum = hex.EncodeToString(sum[:])
	return policy, nil
}

func betterOutcome(left, right Outcome) bool {
	switch {
	case left.NDCGAt10 != right.NDCGAt10:
		return left.NDCGAt10 > right.NDCGAt10
	case left.RecallAt10 != right.RecallAt10:
		return left.RecallAt10 > right.RecallAt10
	case left.LatencyP95 != right.LatencyP95:
		return left.LatencyP95 < right.LatencyP95
	}
	leftParams, _ := canonicalParams(left.Params)
	rightParams, _ := canonicalParams(right.Params)
	if leftParams != rightParams {
		return leftParams < rightParams
	}
	return left.Strategy < right.Strategy
}

func canonicalParams(params PolicyParams) (string, error) {
	if err := validatePolicyParams(params); err != nil {
		return "", err
	}
	content, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("encode policy params: %w", err)
	}
	return string(content), nil
}

func validatePolicyParams(params PolicyParams) error {
	if !allowedPolicyFloat(params.DenseWeight, []float64{0.5, 1, 2}) ||
		!allowedPolicyFloat(params.SparseWeight, []float64{0.5, 1, 2}) {
		return fmt.Errorf("policy weights must be one of 0.5, 1, 2")
	}
	if params.CandidateQuota != 5 && params.CandidateQuota != 10 && params.CandidateQuota != 20 {
		return fmt.Errorf("policy candidate quota must be one of 5, 10, 20")
	}
	if !allowedPolicyFloat(params.MinimumRelevance, []float64{0, 0.25, 0.5}) {
		return fmt.Errorf("policy minimum relevance must be one of 0, 0.25, 0.5")
	}
	return nil
}

func allowedPolicyFloat(value float64, allowed []float64) bool {
	if !finite(value) {
		return false
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func selectableKind(kind QueryKind) bool {
	switch kind {
	case QueryExact, QueryCode, QuerySemantic, QueryMixed:
		return true
	default:
		return false
	}
}

func normalizeEncoderIdentity(identity EncoderIdentity) EncoderIdentity {
	identity.EmbeddingModel = strings.TrimSpace(identity.EmbeddingModel)
	identity.SparseEncoderID = strings.TrimSpace(identity.SparseEncoderID)
	return identity
}
