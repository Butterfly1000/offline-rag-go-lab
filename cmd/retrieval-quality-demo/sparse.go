package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"offline-rag-go-lab/internal/retrievalquality"
)

type sparseCommandReport struct {
	DatasetID      string                      `json:"dataset_id"`
	DatasetVersion string                      `json:"dataset_version"`
	Collection     string                      `json:"collection"`
	EncoderID      string                      `json:"encoder_id"`
	FieldStats     retrievalquality.FieldStats `json:"field_stats"`
	Train          retrievalquality.Report     `json:"train"`
	Validation     retrievalquality.Report     `json:"validation"`
	Passed         bool                        `json:"passed"`
}

func runSparse(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("sparse", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
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
	strategy, err := retrievalquality.NewQdrantSparseStrategy(qdrant, encoder)
	if err != nil {
		return err
	}
	report, err := buildSparseReport(ctx, resources.Dataset, strategy)
	if err != nil {
		return err
	}
	report.DatasetID = resources.Dataset.Manifest.DatasetID
	report.DatasetVersion = resources.Dataset.Manifest.Version
	report.Collection = retrievalquality.RetrievalQualityAlias
	report.EncoderID = resources.EncoderID
	report.FieldStats = stats
	if err := encodeIndented(output, report); err != nil {
		return err
	}
	if !report.Passed {
		return fmt.Errorf("sparse evaluation failed safety gates")
	}
	return nil
}

func buildSparseReport(ctx context.Context, dataset retrievalquality.Dataset, strategy retrievalquality.Strategy) (sparseCommandReport, error) {
	train, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitTrain, strategy, 10)
	if err != nil {
		return sparseCommandReport{}, err
	}
	validation, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitValidation, strategy, 10)
	if err != nil {
		return sparseCommandReport{}, err
	}
	return sparseCommandReport{
		Train: train, Validation: validation,
		Passed: train.Passed && validation.Passed && train.ScopeIsolation == 1 &&
			validation.ScopeIsolation == 1 && train.ForbiddenHitCount == 0 &&
			validation.ForbiddenHitCount == 0,
	}, nil
}
