package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/retrievalquality"
)

type forcedFallbackProof struct {
	Endpoint     string   `json:"endpoint"`
	CaseID       string   `json:"case_id"`
	OriginalIDs  []string `json:"original_ids"`
	FallbackIDs  []string `json:"fallback_ids"`
	Warnings     []string `json:"warnings"`
	SameRRFOrder bool     `json:"same_rrf_order"`
}

type rerankCommandReport struct {
	DatasetID           string                           `json:"dataset_id"`
	DatasetVersion      string                           `json:"dataset_version"`
	Collection          string                           `json:"collection"`
	Model               string                           `json:"model"`
	CandidateLimit      int                              `json:"candidate_limit"`
	Diversity           retrievalquality.DiversityLimits `json:"diversity"`
	HybridValidation    retrievalquality.Report          `json:"hybrid_validation"`
	RerankedValidation  retrievalquality.Report          `json:"reranked_validation"`
	Stats               retrievalquality.RerankStats     `json:"reranker_stats"`
	ForcedFallback      forcedFallbackProof              `json:"forced_fallback"`
	DefaultEnabled      bool                             `json:"default_enabled"`
	DefaultEnableReason string                           `json:"default_enable_reason"`
	Passed              bool                             `json:"passed"`
}

func runRerank(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("rerank", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
	model := flags.String("model", "qwen:7b", "local Ollama reranker model")
	rerankerURL := flags.String("reranker-url", "", "Ollama reranker URL; defaults to project config")
	fallbackURL := flags.String("fallback-url", "http://127.0.0.1:1", "deliberately unavailable local endpoint")
	if err := flags.Parse(args); err != nil {
		return err
	}
	resources, err := loadCommandResources(*configPath, *datasetPath)
	if err != nil {
		return err
	}
	stats, err := retrievalquality.BuildFieldStats(resources.Dataset.Corpus, resources.Tokenizer)
	if err != nil {
		return err
	}
	encoder, err := retrievalquality.NewSparseEncoder(resources.Tokenizer, stats)
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
	hybrid, err := retrievalquality.NewHybridStrategy(
		dense, sparse, retrievalquality.FusionWeights{Dense: 1, Sparse: 1}, retrievalquality.HybridEvaluation,
	)
	if err != nil {
		return err
	}
	if *rerankerURL == "" {
		*rerankerURL = resources.OllamaURL
	}
	ollamaReranker, err := retrievalquality.NewOllamaReranker(*rerankerURL, *model)
	if err != nil {
		return err
	}
	limits := retrievalquality.DiversityLimits{PerDocument: 3, PerHeading: 2}
	reranked, err := retrievalquality.NewRerankedStrategy(hybrid, ollamaReranker, limits)
	if err != nil {
		return err
	}
	report, err := buildRerankReport(ctx, resources.Dataset, hybrid, reranked, *model)
	if err != nil {
		return err
	}
	fallback, err := proveForcedFallback(ctx, resources.Dataset, hybrid, *fallbackURL, *model)
	if err != nil {
		return err
	}
	report.DatasetID = resources.Dataset.Manifest.DatasetID
	report.DatasetVersion = resources.Dataset.Manifest.Version
	report.Collection = retrievalquality.RetrievalQualityAlias
	report.CandidateLimit = 20
	report.Diversity = limits
	report.ForcedFallback = fallback
	report.Passed = report.Passed && fallback.SameRRFOrder
	if err := encodeIndented(output, report); err != nil {
		return err
	}
	if !report.Passed {
		return fmt.Errorf("reranker comparison failed safety or execution gates")
	}
	return nil
}

func buildRerankReport(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	hybrid retrievalquality.Strategy,
	reranked *retrievalquality.RerankedStrategy,
	model string,
) (rerankCommandReport, error) {
	hybridReport, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitValidation, hybrid, 10)
	if err != nil {
		return rerankCommandReport{}, err
	}
	rerankedReport, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitValidation, reranked, 10)
	if err != nil {
		return rerankCommandReport{}, err
	}
	stats := reranked.Stats()
	improved := rerankedReport.MeanNDCGAt10 > hybridReport.MeanNDCGAt10
	defaultEnabled := improved && stats.Calls > 0 && stats.Used == stats.Calls
	reason := "disabled: validation NDCG@10 did not strictly improve"
	if improved && stats.Used != stats.Calls {
		reason = "disabled: validation included reranker fallback"
	} else if defaultEnabled {
		reason = "enabled: validation NDCG@10 strictly improved with no fallback"
	}
	return rerankCommandReport{
		Model:               model,
		HybridValidation:    hybridReport,
		RerankedValidation:  rerankedReport,
		Stats:               stats,
		DefaultEnabled:      defaultEnabled,
		DefaultEnableReason: reason,
		Passed: hybridReport.Passed && rerankedReport.Passed &&
			hybridReport.ScopeIsolation == 1 && rerankedReport.ScopeIsolation == 1 &&
			hybridReport.ForbiddenHitCount == 0 && rerankedReport.ForbiddenHitCount == 0 &&
			stats.Calls == rerankedReport.CaseCount && stats.Used > 0,
	}, nil
}

func proveForcedFallback(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	hybrid retrievalquality.Strategy,
	endpoint, model string,
) (forcedFallbackProof, error) {
	var selected retrievalquality.GoldenCase
	for _, item := range dataset.Cases {
		if item.Split == retrievalquality.SplitValidation && item.Kind == retrievalquality.QueryMixed {
			selected = item
			break
		}
	}
	if selected.CaseID == "" {
		return forcedFallbackProof{}, fmt.Errorf("validation mixed case is required for fallback proof")
	}
	query := retrievalquality.Query{
		CaseID: selected.CaseID, Text: selected.Query,
		KnowledgeScope: selected.KnowledgeScope, Kind: selected.Kind, Limit: 10,
	}
	original, err := hybrid.Search(ctx, query)
	if err != nil {
		return forcedFallbackProof{}, err
	}
	unavailable, err := retrievalquality.NewOllamaReranker(endpoint, model)
	if err != nil {
		return forcedFallbackProof{}, err
	}
	fallback, err := retrievalquality.ApplyReranker(ctx, unavailable, query.Text, original.Candidates)
	if err != nil {
		return forcedFallbackProof{}, err
	}
	if fallback.Used || len(fallback.Warnings) == 0 {
		return forcedFallbackProof{}, fmt.Errorf("forced fallback endpoint unexpectedly returned a valid rerank")
	}
	proof := forcedFallbackProof{
		Endpoint: endpoint, CaseID: selected.CaseID,
		OriginalIDs: candidateIDs(original.Candidates), FallbackIDs: candidateIDs(fallback.Candidates),
		Warnings: fallback.Warnings,
	}
	proof.SameRRFOrder = sameCandidateOrder(original.Candidates, fallback.Candidates)
	return proof, nil
}

func candidateIDs(candidates []retrievalquality.Candidate) []string {
	result := make([]string, len(candidates))
	for index, candidate := range candidates {
		result[index] = candidate.ChunkID
	}
	return result
}

func sameCandidateOrder(left, right []retrievalquality.Candidate) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ChunkID != right[index].ChunkID {
			return false
		}
	}
	return true
}
