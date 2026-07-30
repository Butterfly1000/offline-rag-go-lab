package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"strings"

	"offline-rag-go-lab/internal/memoryitem"
	"offline-rag-go-lab/internal/retrievalquality"
)

type indexStore interface {
	EnsureCollection(context.Context, int) error
	Upsert(context.Context, retrievalquality.IndexedChunk) error
	Inspect(context.Context, int, int) (retrievalquality.CollectionState, error)
	Activate(context.Context) error
}

type indexCommandReport struct {
	DatasetID      string                      `json:"dataset_id"`
	DatasetVersion string                      `json:"dataset_version"`
	Collection     string                      `json:"collection"`
	Alias          string                      `json:"alias"`
	EmbeddingModel string                      `json:"embedding_model"`
	EncoderID      string                      `json:"encoder_id"`
	VectorSize     int                         `json:"vector_size"`
	PointCount     int                         `json:"point_count"`
	FieldStats     retrievalquality.FieldStats `json:"field_stats"`
	Status         string                      `json:"status"`
	AliasActivated bool                        `json:"alias_activated"`
	Applied        bool                        `json:"applied"`
}

func runIndex(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("index", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "config/recent-chat.env", "local project config")
	datasetPath := flags.String("dataset", "internal/retrievalquality/testdata/golden/v1", "versioned dataset directory")
	apply := flags.Bool("apply", false, "write the isolated physical collection")
	activate := flags.Bool("activate", false, "activate the stable alias after verification")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *activate && !*apply {
		return fmt.Errorf("--activate requires --apply")
	}
	resources, err := loadCommandResources(*configPath, *datasetPath)
	if err != nil {
		return err
	}
	if !*apply {
		report := indexCommandReport{
			DatasetID: resources.Dataset.Manifest.DatasetID, DatasetVersion: resources.Dataset.Manifest.Version,
			Collection: retrievalquality.RetrievalQualityCollection, Alias: retrievalquality.RetrievalQualityAlias,
			EmbeddingModel: resources.EmbedModel, EncoderID: resources.EncoderID,
			PointCount: len(resources.Dataset.Corpus), Status: "dry_run", Applied: false,
		}
		return encodeIndented(output, report)
	}
	qdrant, err := retrievalquality.NewQdrant(
		resources.QdrantURL, retrievalquality.RetrievalQualityCollection, retrievalquality.RetrievalQualityAlias,
	)
	if err != nil {
		return err
	}
	report, err := indexDataset(
		ctx, resources.Dataset, qdrant, memoryitem.NewHTTPOllamaEmbedder(resources.OllamaURL),
		resources.Tokenizer, resources.EmbedModel, resources.EncoderID, *activate,
	)
	if err != nil {
		return err
	}
	report.Applied = true
	if report.VectorSize != retrievalquality.RetrievalQualityVectorSize || !strings.EqualFold(report.Status, "green") {
		return fmt.Errorf(
			"verified index uses vector_size=%d status=%q, want %d/green",
			report.VectorSize, report.Status, retrievalquality.RetrievalQualityVectorSize,
		)
	}
	return encodeIndented(output, report)
}

func indexDataset(
	ctx context.Context,
	dataset retrievalquality.Dataset,
	index indexStore,
	embedder retrievalquality.Embedder,
	tokenizer retrievalquality.Tokenizer,
	model, encoderID string,
	activate bool,
) (indexCommandReport, error) {
	if index == nil || embedder == nil || tokenizer == nil {
		return indexCommandReport{}, fmt.Errorf("index, embedder, and tokenizer are required")
	}
	texts := make([]string, len(dataset.Corpus))
	for i, chunk := range dataset.Corpus {
		texts[i] = chunk.Content
	}
	vectors, err := embedder.Embed(ctx, model, texts)
	if err != nil {
		return indexCommandReport{}, fmt.Errorf("embed index corpus: %w", err)
	}
	if len(vectors) != len(dataset.Corpus) || len(vectors) == 0 {
		return indexCommandReport{}, fmt.Errorf("index embedding count or dimension is invalid")
	}
	vectorSize := retrievalquality.RetrievalQualityVectorSize
	for i, vector := range vectors {
		if len(vector) != vectorSize {
			return indexCommandReport{}, fmt.Errorf("index vector %d dimension=%d, want %d", i, len(vector), vectorSize)
		}
		for j, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return indexCommandReport{}, fmt.Errorf("index vector %d value %d must be finite", i, j)
			}
		}
	}
	stats, err := retrievalquality.BuildFieldStats(dataset.Corpus, tokenizer)
	if err != nil {
		return indexCommandReport{}, err
	}
	encoder, err := retrievalquality.NewSparseEncoder(tokenizer, stats)
	if err != nil {
		return indexCommandReport{}, err
	}
	if err := index.EnsureCollection(ctx, vectorSize); err != nil {
		return indexCommandReport{}, err
	}
	for i, chunk := range dataset.Corpus {
		sparse, err := encoder.EncodeChunk(chunk)
		if err != nil {
			return indexCommandReport{}, fmt.Errorf("encode sparse chunk %q: %w", chunk.ChunkID, err)
		}
		if err := index.Upsert(ctx, retrievalquality.IndexedChunk{
			Chunk: chunk, Dense: vectors[i], Sparse: sparse,
			EmbeddingModel: model, EncoderID: encoderID,
		}); err != nil {
			return indexCommandReport{}, fmt.Errorf("upsert chunk %q: %w", chunk.ChunkID, err)
		}
	}
	state, err := index.Inspect(ctx, vectorSize, len(dataset.Corpus))
	if err != nil {
		return indexCommandReport{}, err
	}
	activated := false
	if activate {
		if err := index.Activate(ctx); err != nil {
			return indexCommandReport{}, err
		}
		activated = true
	}
	return indexCommandReport{
		DatasetID: dataset.Manifest.DatasetID, DatasetVersion: dataset.Manifest.Version,
		Collection: retrievalquality.RetrievalQualityCollection, Alias: retrievalquality.RetrievalQualityAlias,
		EmbeddingModel: model, EncoderID: encoderID, VectorSize: vectorSize,
		PointCount: state.PointCount, FieldStats: stats, Status: state.Status,
		AliasActivated: activated,
	}, nil
}

func encodeIndented(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
