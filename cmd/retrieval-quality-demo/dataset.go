package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"offline-rag-go-lab/internal/fileconfig"
	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/retrievalquality"
)

type datasetCommandReport struct {
	DatasetID      string                  `json:"dataset_id"`
	DatasetVersion string                  `json:"dataset_version"`
	CorpusSHA256   string                  `json:"corpus_sha256"`
	CasesSHA256    string                  `json:"cases_sha256"`
	EmbeddingModel string                  `json:"embedding_model"`
	CaseCount      int                     `json:"case_count"`
	Train          retrievalquality.Report `json:"train"`
	Validation     retrievalquality.Report `json:"validation"`
	Passed         bool                    `json:"passed"`
}

func runDataset(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("dataset", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	values, err := fileconfig.Load(*configPath)
	if err != nil {
		return err
	}
	baseURL, err := fileconfig.Required(values, "OLLAMA_BASE_URL")
	if err != nil {
		return err
	}
	model, err := fileconfig.Required(values, "OLLAMA_EMBED_MODEL")
	if err != nil {
		return err
	}
	dataset, err := retrievalquality.LoadDataset(*datasetPath)
	if err != nil {
		return err
	}
	report, err := buildDatasetReport(ctx, dataset, memoryitem.NewHTTPOllamaEmbedder(baseURL), model)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode dataset report: %w", err)
	}
	if !report.Passed {
		return fmt.Errorf("dense dataset baseline failed safety gates")
	}
	return nil
}

func buildDatasetReport(ctx context.Context, dataset retrievalquality.Dataset, embedder retrievalquality.Embedder, model string) (datasetCommandReport, error) {
	strategy, err := retrievalquality.NewDenseMemoryStrategy(ctx, embedder, model, dataset.Corpus)
	if err != nil {
		return datasetCommandReport{}, err
	}
	train, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitTrain, strategy, 10)
	if err != nil {
		return datasetCommandReport{}, err
	}
	validation, err := retrievalquality.Evaluate(ctx, dataset, retrievalquality.SplitValidation, strategy, 10)
	if err != nil {
		return datasetCommandReport{}, err
	}
	return datasetCommandReport{
		DatasetID: dataset.Manifest.DatasetID, DatasetVersion: dataset.Manifest.Version,
		CorpusSHA256: dataset.Manifest.CorpusSHA256, CasesSHA256: dataset.Manifest.CasesSHA256,
		EmbeddingModel: model, CaseCount: train.CaseCount + validation.CaseCount,
		Train: train, Validation: validation,
		Passed: train.Passed && validation.Passed && train.ScopeIsolation == 1 &&
			validation.ScopeIsolation == 1 && train.ForbiddenHitCount == 0 &&
			validation.ForbiddenHitCount == 0,
	}, nil
}
