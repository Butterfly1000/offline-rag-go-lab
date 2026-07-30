package retrievalquality

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PolicyStrategy struct {
	policy     Policy
	strategies map[StrategyName]Strategy
}

func NewPolicyStrategy(policy Policy, strategies map[StrategyName]Strategy) (*PolicyStrategy, error) {
	for _, name := range []StrategyName{StrategyDense, StrategySparse, StrategyHybrid} {
		if strategies[name] == nil {
			return nil, fmt.Errorf("policy strategy %q is required", name)
		}
		calibrator, exists := policy.Calibrators[name]
		if !exists || len(calibrator.Breakpoints) == 0 ||
			len(calibrator.Breakpoints) != len(calibrator.Values) {
			return nil, fmt.Errorf("policy calibrator %q is required", name)
		}
	}
	for kind, route := range policy.Routes {
		if err := validatePolicyParams(route.Params); err != nil {
			return nil, fmt.Errorf("policy route %q: %w", kind, err)
		}
	}
	if err := validatePolicyParams(policy.UnknownRoute.Params); err != nil {
		return nil, fmt.Errorf("policy unknown route: %w", err)
	}
	return &PolicyStrategy{policy: policy, strategies: strategies}, nil
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
		current, err := s.strategies[name].Search(ctx, searchQuery)
		if err == nil {
			result = current
			selected = name
			if index > 0 {
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
