package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/retrievalquality"
)

type strategySplitReport struct {
	Train      retrievalquality.Report `json:"train"`
	Validation retrievalquality.Report `json:"validation"`
}

type hybridTraceExample struct {
	CaseID     string                       `json:"case_id"`
	Query      string                       `json:"query"`
	Candidates []retrievalquality.Candidate `json:"candidates"`
}

type hybridCommandReport struct {
	DatasetID      string                         `json:"dataset_id"`
	DatasetVersion string                         `json:"dataset_version"`
	Collection     string                         `json:"collection"`
	EncoderID      string                         `json:"encoder_id"`
	Weights        retrievalquality.FusionWeights `json:"weights"`
	RankConstant   int                            `json:"rank_constant"`
	Dense          strategySplitReport            `json:"dense"`
	Sparse         strategySplitReport            `json:"sparse"`
	Hybrid         strategySplitReport            `json:"hybrid"`
	TraceExample   hybridTraceExample             `json:"trace_example"`
	Passed         bool                           `json:"passed"`
}

func runHybrid(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("hybrid", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
	denseWeight := flags.Float64("dense-weight", 1, "RRF dense rank weight")
	sparseWeight := flags.Float64("sparse-weight", 1, "RRF sparse rank weight")
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
	weights := retrievalquality.FusionWeights{Dense: *denseWeight, Sparse: *sparseWeight}
	hybrid, err := retrievalquality.NewHybridStrategy(dense, sparse, weights, retrievalquality.HybridEvaluation)
	if err != nil {
		return err
	}
	report, err := buildHybridReport(ctx, resources.Dataset, dense, sparse, hybrid)
	if err != nil {
		return err
	}
	report.DatasetID = resources.Dataset.Manifest.DatasetID
	report.DatasetVersion = resources.Dataset.Manifest.Version
	report.Collection = retrievalquality.RetrievalQualityAlias
	report.EncoderID = resources.EncoderID
	report.Weights = weights
	report.RankConstant = 60
	if err := encodeIndented(output, report); err != nil {
		return err
	}
	if !report.Passed {
		return fmt.Errorf("hybrid comparison failed safety gates")
	}
	return nil
}

func buildHybridReport(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	dense, sparse, hybrid retrievalquality.Strategy,
) (hybridCommandReport, error) {
	denseReports, err := evaluateStrategySplits(ctx, dataset, dense)
	if err != nil {
		return hybridCommandReport{}, err
	}
	sparseReports, err := evaluateStrategySplits(ctx, dataset, sparse)
	if err != nil {
		return hybridCommandReport{}, err
	}
	hybridReports, err := evaluateStrategySplits(ctx, dataset, hybrid)
	if err != nil {
		return hybridCommandReport{}, err
	}
	var exampleCase retrievalquality.GoldenCase
	for _, item := range dataset.Cases {
		if item.Split == retrievalquality.SplitValidation && item.Kind == retrievalquality.QueryMixed {
			exampleCase = item
			break
		}
	}
	if exampleCase.CaseID == "" {
		return hybridCommandReport{}, fmt.Errorf("validation mixed case is required for hybrid trace")
	}
	example, err := hybrid.Search(ctx, retrievalquality.Query{
		CaseID: exampleCase.CaseID, Text: exampleCase.Query,
		KnowledgeScope: exampleCase.KnowledgeScope, Kind: exampleCase.Kind, Limit: 5,
	})
	if err != nil {
		return hybridCommandReport{}, err
	}
	report := hybridCommandReport{
		Dense: denseReports, Sparse: sparseReports, Hybrid: hybridReports,
		TraceExample: hybridTraceExample{
			CaseID: exampleCase.CaseID, Query: exampleCase.Query, Candidates: example.Candidates,
		},
	}
	report.Passed = splitReportsSafe(report.Dense) && splitReportsSafe(report.Sparse) &&
		splitReportsSafe(report.Hybrid)
	return report, nil
}

func evaluateStrategySplits(ctx context.Context, dataset retrievalquality.Dataset, strategy retrievalquality.Strategy) (strategySplitReport, error) {
	train, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitTrain, strategy, 10)
	if err != nil {
		return strategySplitReport{}, err
	}
	validation, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitValidation, strategy, 10)
	if err != nil {
		return strategySplitReport{}, err
	}
	return strategySplitReport{Train: train, Validation: validation}, nil
}

func splitReportsSafe(report strategySplitReport) bool {
	return report.Train.Passed && report.Validation.Passed &&
		report.Train.ScopeIsolation == 1 && report.Validation.ScopeIsolation == 1 &&
		report.Train.ForbiddenHitCount == 0 && report.Validation.ForbiddenHitCount == 0
}
