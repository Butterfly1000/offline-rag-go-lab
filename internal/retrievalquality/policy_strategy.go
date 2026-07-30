package retrievalquality

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type PolicyStrategy struct {
	policy  Policy
	dense   Strategy
	sparse  Strategy
	hybrids map[string]Strategy
}

func NewPolicyStrategy(policy Policy, dense, sparse Strategy) (*PolicyStrategy, error) {
	if dense == nil || sparse == nil {
		return nil, fmt.Errorf("policy dense and sparse strategies are required")
	}
	for _, name := range []StrategyName{StrategyDense, StrategySparse, StrategyHybrid} {
		calibrator, exists := policy.Calibrators[name]
		if !exists {
			return nil, fmt.Errorf("policy calibrator %q is required", name)
		}
		if err := validatePolicyCalibrator(calibrator); err != nil {
			return nil, fmt.Errorf("policy calibrator %q: %w", name, err)
		}
	}
	requiredKinds := map[QueryKind]bool{
		QueryExact: true, QueryCode: true, QuerySemantic: true, QueryMixed: true,
	}
	for kind, route := range policy.Routes {
		if !requiredKinds[kind] {
			return nil, fmt.Errorf("policy route kind %q is invalid", kind)
		}
		delete(requiredKinds, kind)
		if !validPolicyStrategyName(route.Strategy) {
			return nil, fmt.Errorf("policy route %q strategy %q is invalid", kind, route.Strategy)
		}
		if err := validatePolicyParams(route.Params); err != nil {
			return nil, fmt.Errorf("policy route %q: %w", kind, err)
		}
		routeCalibrator, exists := policy.RouteCalibrators[kind]
		if !exists {
			return nil, fmt.Errorf("policy route calibrator %q is required", kind)
		}
		if err := validatePolicyCalibrator(routeCalibrator); err != nil {
			return nil, fmt.Errorf("policy route calibrator %q: %w", kind, err)
		}
	}
	for kind := range requiredKinds {
		return nil, fmt.Errorf("policy route %q is required", kind)
	}
	if len(policy.RouteCalibrators) != len(policy.Routes) {
		return nil, fmt.Errorf("policy route calibrators must match policy routes")
	}
	if !validPolicyStrategyName(policy.UnknownRoute.Strategy) {
		return nil, fmt.Errorf("policy unknown strategy %q is invalid", policy.UnknownRoute.Strategy)
	}
	if err := validatePolicyParams(policy.UnknownRoute.Params); err != nil {
		return nil, fmt.Errorf("policy unknown route: %w", err)
	}
	result := &PolicyStrategy{
		policy: policy, dense: dense, sparse: sparse,
		hybrids: map[string]Strategy{},
	}
	routes := make([]PolicyRoute, 0, len(policy.Routes)+1)
	for _, route := range policy.Routes {
		routes = append(routes, route)
	}
	routes = append(routes, policy.UnknownRoute)
	for _, route := range routes {
		key := runtimeWeightsKey(route.Params)
		if result.hybrids[key] != nil {
			continue
		}
		hybrid, err := NewHybridStrategy(
			dense, sparse,
			FusionWeights{Dense: route.Params.DenseWeight, Sparse: route.Params.SparseWeight},
			HybridRuntime,
		)
		if err != nil {
			return nil, err
		}
		result.hybrids[key] = hybrid
	}
	return result, nil
}

func (s *PolicyStrategy) Name() string { return "calibrated_retrieval_policy" }

func (s *PolicyStrategy) Search(ctx context.Context, query Query) (SearchResult, error) {
	if s == nil {
		return SearchResult{}, fmt.Errorf("policy strategy is not initialized")
	}
	query.Text = strings.TrimSpace(query.Text)
	query.KnowledgeScope = strings.TrimSpace(query.KnowledgeScope)
	if query.Text == "" || query.KnowledgeScope == "" || query.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("policy query text, scope, and positive limit are required")
	}
	started := time.Now()
	route := s.policy.Route(query.Kind)
	searchQuery := query
	searchQuery.Limit = route.Params.CandidateQuota
	order := fallbackOrder(route.Strategy)
	var result SearchResult
	var selected StrategyName
	var warnings []string
	for index, name := range order {
		strategy, err := s.strategy(name, route.Params)
		if err != nil {
			return SearchResult{}, err
		}
		current, err := strategy.Search(ctx, searchQuery)
		if err == nil {
			result = current
			selected = name
			if name == StrategyHybrid && isDenseFallback(current.Candidates) {
				selected = StrategyDense
			}
			if index > 0 || selected != name {
				warnings = append(warnings, fmt.Sprintf(
					"policy fallback %s->%s", route.Strategy, selected,
				))
			}
			break
		}
		if !IsInfrastructure(err) {
			return SearchResult{}, err
		}
		if index == len(order)-1 {
			return SearchResult{}, fmt.Errorf("policy strategies unavailable: %w", err)
		}
	}
	if selected == "" {
		return SearchResult{}, fmt.Errorf("policy did not select a retrieval strategy")
	}
	if err := validateCandidateLeg("policy "+string(selected), result.Candidates); err != nil {
		return SearchResult{}, &IntegrityError{Err: err}
	}
	candidates := cloneCandidates(result.Candidates)
	calibrator := s.policy.Calibrators[selected]
	if selected == route.Strategy {
		if routeCalibrator, exists := s.policy.RouteCalibrators[query.Kind]; exists {
			calibrator = routeCalibrator
		}
	}
	for index := range candidates {
		if candidates[index].KnowledgeScope != query.KnowledgeScope {
			return SearchResult{}, &IntegrityError{Err: fmt.Errorf(
				"policy candidate %q belongs to scope %q, expected %q",
				candidates[index].ChunkID, candidates[index].KnowledgeScope, query.KnowledgeScope,
			)}
		}
		if candidates[index].Trace == nil {
			candidates[index].Trace = map[string]float64{}
		}
		candidates[index].Trace["calibrated_relevance"] = calibrator.Predict(candidates[index].Score)
		candidates[index].Reason = "policy_" + string(selected)
	}
	abstained := result.Abstained
	if len(candidates) == 0 || candidates[0].Trace["calibrated_relevance"] < route.Params.MinimumRelevance {
		abstained = true
		candidates = []Candidate{}
	}
	if len(candidates) > query.Limit {
		candidates = candidates[:query.Limit]
	}
	warnings = append(result.Warnings, warnings...)
	duration := time.Since(started)
	if result.Duration > duration {
		duration = result.Duration
	}
	return SearchResult{
		Candidates: candidates, Abstained: abstained,
		Duration: duration, Warnings: warnings,
	}, nil
}

func validPolicyStrategyName(name StrategyName) bool {
	return name == StrategyDense || name == StrategySparse || name == StrategyHybrid
}

func validatePolicyCalibrator(calibrator Calibrator) error {
	if len(calibrator.Breakpoints) == 0 ||
		len(calibrator.Breakpoints) != len(calibrator.Values) {
		return fmt.Errorf("breakpoints and values must have the same non-zero length")
	}
	for index := range calibrator.Breakpoints {
		if !finite(calibrator.Breakpoints[index]) || !finite(calibrator.Values[index]) ||
			calibrator.Values[index] < 0 || calibrator.Values[index] > 1 {
			return fmt.Errorf("item %d must be finite with value in [0,1]", index)
		}
		if index > 0 && (calibrator.Breakpoints[index] <= calibrator.Breakpoints[index-1] ||
			calibrator.Values[index] < calibrator.Values[index-1]) {
			return fmt.Errorf("breakpoints must increase and values must not decrease")
		}
	}
	return nil
}

func isDenseFallback(candidates []Candidate) bool {
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if candidate.Reason != "dense_fallback" {
			return false
		}
	}
	return true
}

func (s *PolicyStrategy) strategy(name StrategyName, params PolicyParams) (Strategy, error) {
	switch name {
	case StrategyDense:
		return s.dense, nil
	case StrategySparse:
		return s.sparse, nil
	case StrategyHybrid:
		strategy := s.hybrids[runtimeWeightsKey(params)]
		if strategy == nil {
			return nil, fmt.Errorf("policy hybrid weights are not initialized")
		}
		return strategy, nil
	default:
		return nil, fmt.Errorf("policy strategy %q is invalid", name)
	}
}

func runtimeWeightsKey(params PolicyParams) string {
	return fmt.Sprintf("%.17g/%.17g", params.DenseWeight, params.SparseWeight)
}

func LoadPolicyStrategy(
	path string,
	dataset Dataset,
	identity EncoderIdentity,
	dense, sparse Strategy,
) (Strategy, []string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil, fmt.Errorf("policy path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fallback, fallbackErr := NewHybridStrategy(
				dense, sparse, FusionWeights{Dense: 1, Sparse: 1}, HybridRuntime,
			)
			if fallbackErr != nil {
				return nil, nil, fallbackErr
			}
			return fallback, []string{"policy artifact missing; using equal-weight RRF"}, nil
		}
		return nil, nil, fmt.Errorf("open policy artifact: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return nil, nil, fmt.Errorf("decode policy artifact: %w", err)
	}
	if err := ensurePolicyEOF(decoder); err != nil {
		return nil, nil, err
	}
	if err := policy.ValidateIdentity(dataset, identity); err != nil {
		return nil, nil, err
	}
	strategy, err := NewPolicyStrategy(policy, dense, sparse)
	if err != nil {
		return nil, nil, err
	}
	return strategy, nil, nil
}

func ensurePolicyEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("policy artifact contains trailing JSON")
		}
		return fmt.Errorf("decode policy artifact trailer: %w", err)
	}
	return nil
}

func fallbackOrder(strategy StrategyName) []StrategyName {
	switch strategy {
	case StrategySparse:
		return []StrategyName{StrategySparse, StrategyHybrid, StrategyDense}
	case StrategyHybrid:
		return []StrategyName{StrategyHybrid, StrategyDense}
	default:
		return []StrategyName{StrategyDense}
	}
}
