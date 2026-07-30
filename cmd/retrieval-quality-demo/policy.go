package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/retrievalquality"
)

type calibrationMetrics struct {
	SampleCount int     `json:"sample_count"`
	BrierScore  float64 `json:"brier_score"`
	ECE         float64 `json:"ece"`
}

type calibrationCommandReport struct {
	Calibrator retrievalquality.Calibrator `json:"calibrator"`
	Train      calibrationMetrics          `json:"train"`
	Validation calibrationMetrics          `json:"validation"`
}

type policyCommandReport struct {
	DatasetID            string                                                     `json:"dataset_id"`
	DatasetVersion       string                                                     `json:"dataset_version"`
	Collection           string                                                     `json:"collection"`
	EncoderIdentity      retrievalquality.EncoderIdentity                           `json:"encoder_identity"`
	FullGridCount        int                                                        `json:"full_grid_count"`
	EvaluatedGridCount   int                                                        `json:"evaluated_grid_count"`
	TrainingOutcomeCount int                                                        `json:"training_outcome_count"`
	RerankerExcluded     string                                                     `json:"reranker_excluded_reason"`
	Calibration          map[retrievalquality.StrategyName]calibrationCommandReport `json:"calibration"`
	Policy               retrievalquality.Policy                                    `json:"policy"`
	DenseValidation      retrievalquality.Report                                    `json:"dense_validation"`
	PolicyValidation     retrievalquality.Report                                    `json:"policy_validation"`
	Regression           retrievalquality.RegressionResult                          `json:"regression"`
	ArtifactPath         string                                                     `json:"artifact_path"`
	Passed               bool                                                       `json:"passed"`
}

type cachedRawStrategy struct {
	name  string
	cache map[string]retrievalquality.SearchResult
}

func (s cachedRawStrategy) Name() string { return s.name }

func (s cachedRawStrategy) Search(_ context.Context, query retrievalquality.Query) (retrievalquality.SearchResult, error) {
	result, exists := s.cache[query.CaseID]
	if !exists {
		return retrievalquality.SearchResult{}, fmt.Errorf("cached case %q is missing", query.CaseID)
	}
	result.Candidates = cloneCommandCandidates(result.Candidates)
	if len(result.Candidates) > query.Limit {
		result.Candidates = result.Candidates[:query.Limit]
	}
	result.Warnings = append([]string(nil), result.Warnings...)
	return result, nil
}

type cachedConfiguredStrategy struct {
	name             string
	cache            map[string]retrievalquality.SearchResult
	calibrator       retrievalquality.Calibrator
	quota            int
	minimumRelevance float64
}

func (s cachedConfiguredStrategy) Name() string { return s.name }

func (s cachedConfiguredStrategy) Search(
	_ context.Context,
	query retrievalquality.Query,
) (retrievalquality.SearchResult, error) {
	result, exists := s.cache[query.CaseID]
	if !exists {
		return retrievalquality.SearchResult{}, fmt.Errorf("cached configured case %q is missing", query.CaseID)
	}
	result.Candidates = cloneCommandCandidates(result.Candidates)
	if len(result.Candidates) > s.quota {
		result.Candidates = result.Candidates[:s.quota]
	}
	if len(result.Candidates) == 0 ||
		s.calibrator.Predict(result.Candidates[0].Score) < s.minimumRelevance {
		result.Candidates = []retrievalquality.Candidate{}
		result.Abstained = true
	}
	if len(result.Candidates) > query.Limit {
		result.Candidates = result.Candidates[:query.Limit]
	}
	result.Warnings = append([]string(nil), result.Warnings...)
	return result, nil
}

type kindRoutedStrategy struct {
	name       string
	byKind     map[retrievalquality.QueryKind]retrievalquality.Strategy
	defaultOne retrievalquality.Strategy
}

func (s kindRoutedStrategy) Name() string { return s.name }

func (s kindRoutedStrategy) Search(
	ctx context.Context,
	query retrievalquality.Query,
) (retrievalquality.SearchResult, error) {
	if strategy := s.byKind[query.Kind]; strategy != nil {
		return strategy.Search(ctx, query)
	}
	return s.defaultOne.Search(ctx, query)
}

func runPolicy(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("policy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
	artifactPath := flags.String("policy-output", ".cache/retrieval-quality/policy-v1.json", "ignored local policy artifact")
	if err := flags.Parse(args); err != nil {
		return err
	}
	resources, err := loadCommandResources(*configPath, *datasetPath)
	if err != nil {
		return err
	}
	fieldStats, err := retrievalquality.BuildFieldStats(resources.Dataset.Corpus, resources.Tokenizer)
	if err != nil {
		return err
	}
	encoder, err := retrievalquality.NewSparseEncoder(resources.Tokenizer, fieldStats)
	if err != nil {
		return err
	}
	qdrant, err := retrievalquality.NewQdrant(
		resources.QdrantURL, retrievalquality.RetrievalQualityCollection, retrievalquality.RetrievalQualityAlias,
	)
	if err != nil {
		return err
	}
	qdrant.SetExpectedIdentity(resources.EmbedModel, resources.EncoderID)
	dense, err := retrievalquality.NewQdrantDenseStrategy(
		qdrant, memoryitem.NewHTTPOllamaEmbedder(resources.OllamaURL), resources.EmbedModel,
	)
	if err != nil {
		return err
	}
	sparse, err := retrievalquality.NewQdrantSparseStrategy(qdrant, encoder)
	if err != nil {
		return err
	}
	identity := retrievalquality.EncoderIdentity{
		EmbeddingModel: resources.EmbedModel, SparseEncoderID: resources.EncoderID,
	}
	report, err := buildPolicyCommandReport(
		ctx, resources.Dataset, dense, sparse, identity, *artifactPath,
	)
	if err != nil {
		return err
	}
	report.DatasetID = resources.Dataset.Manifest.DatasetID
	report.DatasetVersion = resources.Dataset.Manifest.Version
	report.Collection = retrievalquality.RetrievalQualityAlias
	if err := encodeIndented(output, report); err != nil {
		return err
	}
	if !report.Passed {
		return fmt.Errorf("policy validation failed regression gates")
	}
	return nil
}

func buildPolicyCommandReport(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	dense, sparse retrievalquality.Strategy,
	identity retrievalquality.EncoderIdentity,
	artifactPath string,
) (policyCommandReport, error) {
	denseTrain, err := cachePolicySplit(ctx, dataset, retrievalquality.SplitTrain, dense, 20)
	if err != nil {
		return policyCommandReport{}, err
	}
	denseValidation, err := cachePolicySplit(ctx, dataset, retrievalquality.SplitValidation, dense, 20)
	if err != nil {
		return policyCommandReport{}, err
	}
	sparseTrain, err := cachePolicySplit(ctx, dataset, retrievalquality.SplitTrain, sparse, 20)
	if err != nil {
		return policyCommandReport{}, err
	}
	sparseValidation, err := cachePolicySplit(ctx, dataset, retrievalquality.SplitValidation, sparse, 20)
	if err != nil {
		return policyCommandReport{}, err
	}
	denseCalibration, err := fitCachedCalibration(ctx, dataset, denseTrain, denseValidation, "dense")
	if err != nil {
		return policyCommandReport{}, err
	}
	sparseCalibration, err := fitCachedCalibration(ctx, dataset, sparseTrain, sparseValidation, "sparse")
	if err != nil {
		return policyCommandReport{}, err
	}

	type hybridCache struct {
		strategy   retrievalquality.Strategy
		train      map[string]retrievalquality.SearchResult
		calibrator retrievalquality.Calibrator
	}
	hybrids := map[string]hybridCache{}
	for _, denseWeight := range []float64{0.5, 1, 2} {
		for _, sparseWeight := range []float64{0.5, 1, 2} {
			hybrid, err := retrievalquality.NewHybridStrategy(
				dense, sparse,
				retrievalquality.FusionWeights{Dense: denseWeight, Sparse: sparseWeight},
				retrievalquality.HybridEvaluation,
			)
			if err != nil {
				return policyCommandReport{}, err
			}
			trainCache, err := cachePolicySplit(ctx, dataset, retrievalquality.SplitTrain, hybrid, 20)
			if err != nil {
				return policyCommandReport{}, err
			}
			trainSamples, err := retrievalquality.CollectCalibrationSamples(
				ctx, dataset, retrievalquality.SplitTrain,
				cachedRawStrategy{name: "hybrid_train_cache", cache: trainCache}, 20,
			)
			if err != nil {
				return policyCommandReport{}, err
			}
			calibrator, err := retrievalquality.FitIsotonic(trainSamples)
			if err != nil {
				return policyCommandReport{}, err
			}
			hybrids[policyWeightsKey(denseWeight, sparseWeight)] = hybridCache{
				strategy: hybrid, train: trainCache, calibrator: calibrator,
			}
		}
	}
	equalHybrid := hybrids[policyWeightsKey(1, 1)]
	equalValidation, err := cachePolicySplit(
		ctx, dataset, retrievalquality.SplitValidation, equalHybrid.strategy, 20,
	)
	if err != nil {
		return policyCommandReport{}, err
	}
	hybridCalibration, err := fitCachedCalibration(
		ctx, dataset, equalHybrid.train, equalValidation, "hybrid",
	)
	if err != nil {
		return policyCommandReport{}, err
	}

	grid := evaluatedPolicyGrid()
	var outcomes []retrievalquality.Outcome
	for _, params := range grid {
		hybridCache := hybrids[policyWeightsKey(params.DenseWeight, params.SparseWeight)]
		hybridReport, err := retrievalquality.Evaluate(
			ctx, dataset, retrievalquality.SplitTrain,
			cachedConfiguredStrategy{
				name: "hybrid_grid", cache: hybridCache.train,
				calibrator: hybridCache.calibrator, quota: params.CandidateQuota,
				minimumRelevance: params.MinimumRelevance,
			}, 10,
		)
		if err != nil {
			return policyCommandReport{}, err
		}
		items, err := outcomesFromTrainReport(
			hybridReport, retrievalquality.StrategyHybrid, params, identity,
		)
		if err != nil {
			return policyCommandReport{}, err
		}
		outcomes = append(outcomes, items...)
		if params.DenseWeight == 1 && params.SparseWeight == 1 {
			for _, candidate := range []struct {
				name       retrievalquality.StrategyName
				cache      map[string]retrievalquality.SearchResult
				calibrator retrievalquality.Calibrator
			}{
				{name: retrievalquality.StrategyDense, cache: denseTrain, calibrator: denseCalibration.Calibrator},
				{name: retrievalquality.StrategySparse, cache: sparseTrain, calibrator: sparseCalibration.Calibrator},
			} {
				strategyReport, err := retrievalquality.Evaluate(
					ctx, dataset, retrievalquality.SplitTrain,
					cachedConfiguredStrategy{
						name: string(candidate.name) + "_grid", cache: candidate.cache,
						calibrator: candidate.calibrator, quota: params.CandidateQuota,
						minimumRelevance: params.MinimumRelevance,
					}, 10,
				)
				if err != nil {
					return policyCommandReport{}, err
				}
				items, err := outcomesFromTrainReport(strategyReport, candidate.name, params, identity)
				if err != nil {
					return policyCommandReport{}, err
				}
				outcomes = append(outcomes, items...)
			}
		}
	}
	policy, err := retrievalquality.SelectPolicy(dataset, outcomes, grid)
	if err != nil {
		return policyCommandReport{}, err
	}
	policy.Calibrators = map[retrievalquality.StrategyName]retrievalquality.Calibrator{
		retrievalquality.StrategyDense:  denseCalibration.Calibrator,
		retrievalquality.StrategySparse: sparseCalibration.Calibrator,
		retrievalquality.StrategyHybrid: hybridCalibration.Calibrator,
	}
	policy.RouteCalibrators = map[retrievalquality.QueryKind]retrievalquality.Calibrator{}
	for kind, route := range policy.Routes {
		switch route.Strategy {
		case retrievalquality.StrategyDense:
			policy.RouteCalibrators[kind] = denseCalibration.Calibrator
		case retrievalquality.StrategySparse:
			policy.RouteCalibrators[kind] = sparseCalibration.Calibrator
		case retrievalquality.StrategyHybrid:
			policy.RouteCalibrators[kind] = hybrids[policyWeightsKey(route.Params.DenseWeight, route.Params.SparseWeight)].calibrator
		}
	}
	policy, err = retrievalquality.FinalizePolicy(policy)
	if err != nil {
		return policyCommandReport{}, err
	}
	if err := policy.ValidateIdentity(dataset, identity); err != nil {
		return policyCommandReport{}, err
	}

	selectedHybridValidation := map[string]map[string]retrievalquality.SearchResult{}
	hybridByKind := map[retrievalquality.QueryKind]retrievalquality.Strategy{}
	for kind, route := range policy.Routes {
		if route.Strategy != retrievalquality.StrategyHybrid {
			continue
		}
		key := policyWeightsKey(route.Params.DenseWeight, route.Params.SparseWeight)
		cache, exists := selectedHybridValidation[key]
		if !exists {
			cache, err = cachePolicySplit(
				ctx, dataset, retrievalquality.SplitValidation, hybrids[key].strategy, 20,
			)
			if err != nil {
				return policyCommandReport{}, err
			}
			selectedHybridValidation[key] = cache
		}
		hybridByKind[kind] = cachedRawStrategy{name: "selected_hybrid_validation", cache: cache}
	}
	validationStrategies := map[retrievalquality.StrategyName]retrievalquality.Strategy{
		retrievalquality.StrategyDense:  cachedRawStrategy{name: "dense_validation", cache: denseValidation},
		retrievalquality.StrategySparse: cachedRawStrategy{name: "sparse_validation", cache: sparseValidation},
		retrievalquality.StrategyHybrid: kindRoutedStrategy{
			name: "hybrid_validation_router", byKind: hybridByKind,
			defaultOne: cachedRawStrategy{name: "equal_hybrid_validation", cache: equalValidation},
		},
	}
	policyStrategy, err := retrievalquality.NewPolicyStrategy(policy, validationStrategies)
	if err != nil {
		return policyCommandReport{}, err
	}
	denseValidationReport, err := retrievalquality.Evaluate(
		ctx, dataset, retrievalquality.SplitValidation,
		validationStrategies[retrievalquality.StrategyDense], 10,
	)
	if err != nil {
		return policyCommandReport{}, err
	}
	policyValidationReport, err := retrievalquality.Evaluate(
		ctx, dataset, retrievalquality.SplitValidation, policyStrategy, 10,
	)
	if err != nil {
		return policyCommandReport{}, err
	}
	regression := retrievalquality.CheckRegression(denseValidationReport, policyValidationReport, 3)
	if err := writePolicyArtifact(artifactPath, policy); err != nil {
		return policyCommandReport{}, err
	}
	return policyCommandReport{
		EncoderIdentity:    identity,
		FullGridCount:      len(retrievalquality.DefaultPolicyGrid()),
		EvaluatedGridCount: len(grid), TrainingOutcomeCount: len(outcomes),
		RerankerExcluded: "L37 validation default_enabled=false; rerank=true remains checked in but is not viable",
		Calibration: map[retrievalquality.StrategyName]calibrationCommandReport{
			retrievalquality.StrategyDense:  denseCalibration,
			retrievalquality.StrategySparse: sparseCalibration,
			retrievalquality.StrategyHybrid: hybridCalibration,
		},
		Policy: policy, DenseValidation: denseValidationReport,
		PolicyValidation: policyValidationReport, Regression: regression,
		ArtifactPath: artifactPath, Passed: regression.Passed,
	}, nil
}

func evaluatedPolicyGrid() []retrievalquality.PolicyParams {
	full := retrievalquality.DefaultPolicyGrid()
	result := make([]retrievalquality.PolicyParams, 0, len(full)/2)
	for _, params := range full {
		if !params.RerankEnabled {
			result = append(result, params)
		}
	}
	return result
}

func outcomesFromTrainReport(
	report retrievalquality.Report,
	strategy retrievalquality.StrategyName,
	params retrievalquality.PolicyParams,
	identity retrievalquality.EncoderIdentity,
) ([]retrievalquality.Outcome, error) {
	if report.Split != retrievalquality.SplitTrain {
		return nil, fmt.Errorf("policy selection accepts train reports only")
	}
	result := make([]retrievalquality.Outcome, 0, 4)
	for _, kind := range []retrievalquality.QueryKind{
		retrievalquality.QueryExact, retrievalquality.QueryCode,
		retrievalquality.QuerySemantic, retrievalquality.QueryMixed,
	} {
		item, exists := report.ByKind[kind]
		if !exists {
			return nil, fmt.Errorf("train report is missing query kind %q", kind)
		}
		result = append(result, retrievalquality.Outcome{
			Split: retrievalquality.SplitTrain, Kind: kind, Strategy: strategy, Params: params,
			NDCGAt10: item.MeanNDCGAt10, RecallAt10: item.MeanRecallAt10,
			LatencyP95: report.LatencyP95, EncoderIdentity: identity,
		})
	}
	return result, nil
}

func cachePolicySplit(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	split retrievalquality.Split,
	strategy retrievalquality.Strategy,
	limit int,
) (map[string]retrievalquality.SearchResult, error) {
	cases := append([]retrievalquality.GoldenCase(nil), dataset.Cases...)
	sort.Slice(cases, func(i, j int) bool { return cases[i].CaseID < cases[j].CaseID })
	result := map[string]retrievalquality.SearchResult{}
	for _, golden := range cases {
		if golden.Split != split {
			continue
		}
		search, err := strategy.Search(ctx, retrievalquality.Query{
			CaseID: golden.CaseID, Text: golden.Query,
			KnowledgeScope: golden.KnowledgeScope, Kind: golden.Kind, Limit: limit,
		})
		if err != nil {
			return nil, fmt.Errorf("cache %s case %s: %w", strategy.Name(), golden.CaseID, err)
		}
		result[golden.CaseID] = search
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("cache split %q produced no cases", split)
	}
	return result, nil
}

func fitCachedCalibration(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	train, validation map[string]retrievalquality.SearchResult,
	name string,
) (calibrationCommandReport, error) {
	trainSamples, err := retrievalquality.CollectCalibrationSamples(
		ctx, dataset, retrievalquality.SplitTrain,
		cachedRawStrategy{name: name + "_train_calibration", cache: train}, 20,
	)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	validationSamples, err := retrievalquality.CollectCalibrationSamples(
		ctx, dataset, retrievalquality.SplitValidation,
		cachedRawStrategy{name: name + "_validation_calibration", cache: validation}, 20,
	)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	calibrator, err := retrievalquality.FitIsotonic(trainSamples)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	trainBrier, err := retrievalquality.BrierScore(trainSamples, calibrator)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	validationBrier, err := retrievalquality.BrierScore(validationSamples, calibrator)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	trainECE, err := retrievalquality.ExpectedCalibrationError(trainSamples, calibrator, 10)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	validationECE, err := retrievalquality.ExpectedCalibrationError(validationSamples, calibrator, 10)
	if err != nil {
		return calibrationCommandReport{}, err
	}
	return calibrationCommandReport{
		Calibrator: calibrator,
		Train: calibrationMetrics{
			SampleCount: len(trainSamples), BrierScore: trainBrier, ECE: trainECE,
		},
		Validation: calibrationMetrics{
			SampleCount: len(validationSamples), BrierScore: validationBrier, ECE: validationECE,
		},
	}, nil
}

func policyWeightsKey(denseWeight, sparseWeight float64) string {
	return fmt.Sprintf("%.1f/%.1f", denseWeight, sparseWeight)
}

func cloneCommandCandidates(input []retrievalquality.Candidate) []retrievalquality.Candidate {
	result := make([]retrievalquality.Candidate, len(input))
	for index, candidate := range input {
		result[index] = candidate
		if candidate.Trace != nil {
			result[index].Trace = make(map[string]float64, len(candidate.Trace))
			for key, value := range candidate.Trace {
				result[index].Trace[key] = value
			}
		}
	}
	return result
}

func writePolicyArtifact(path string, policy retrievalquality.Policy) error {
	path = filepath.Clean(path)
	if path == "." || filepath.IsAbs(path) {
		return fmt.Errorf("policy artifact must be a project-relative file")
	}
	content, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("encode policy artifact: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create policy artifact directory: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write policy artifact: %w", err)
	}
	return nil
}
