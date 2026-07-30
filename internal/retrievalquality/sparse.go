package retrievalquality

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"offline-rag-go-lab/internal/tokenizerdemo"
)

const (
	sparseK1            = 1.2
	sparseB             = 0.75
	sparseTitleWeight   = 3.0
	sparseHeadingWeight = 2.0
	sparseSourceWeight  = 1.5
	sparseBodyWeight    = 1.0
)

type Tokenizer interface {
	TokenIDs(string) ([]int, error)
}

type QwenTokenizer struct {
	counter *tokenizerdemo.Counter
}

func NewQwenTokenizer(counter *tokenizerdemo.Counter) (*QwenTokenizer, error) {
	if counter == nil {
		return nil, fmt.Errorf("Qwen tokenizer counter is required")
	}
	return &QwenTokenizer{counter: counter}, nil
}

func (t *QwenTokenizer) TokenIDs(text string) ([]int, error) {
	if t == nil || t.counter == nil {
		return nil, fmt.Errorf("Qwen tokenizer is not initialized")
	}
	_, _, ids, err := t.counter.CountText(text)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

type FieldStats struct {
	AverageTitle   float64 `json:"average_title"`
	AverageHeading float64 `json:"average_heading"`
	AverageSource  float64 `json:"average_source"`
	AverageBody    float64 `json:"average_body"`
}

type SparseVector struct {
	Indices []uint32  `json:"indices"`
	Values  []float32 `json:"values"`
}

type SparseEncoder struct {
	tokenizer Tokenizer
	stats     FieldStats
}

func BuildFieldStats(chunks []Chunk, tokenizer Tokenizer) (FieldStats, error) {
	if len(chunks) == 0 || tokenizer == nil {
		return FieldStats{}, fmt.Errorf("sparse corpus and tokenizer are required")
	}
	var totals [4]int
	for index, chunk := range chunks {
		fields := []string{chunk.Title, chunk.HeadingPath, chunk.SourceRef, chunk.Content}
		for fieldIndex, text := range fields {
			ids, err := tokenizer.TokenIDs(text)
			if err != nil {
				return FieldStats{}, fmt.Errorf("tokenize chunk %d field %d: %w", index, fieldIndex, err)
			}
			if err := validateTokenIDs(ids); err != nil {
				return FieldStats{}, fmt.Errorf("chunk %d field %d: %w", index, fieldIndex, err)
			}
			totals[fieldIndex] += len(ids)
		}
	}
	count := float64(len(chunks))
	stats := FieldStats{
		AverageTitle: float64(totals[0]) / count, AverageHeading: float64(totals[1]) / count,
		AverageSource: float64(totals[2]) / count, AverageBody: float64(totals[3]) / count,
	}
	stats.ensureNonZero()
	return stats, nil
}

func NewSparseEncoder(tokenizer Tokenizer, stats FieldStats) (*SparseEncoder, error) {
	if tokenizer == nil {
		return nil, fmt.Errorf("sparse tokenizer is required")
	}
	if err := stats.validate(); err != nil {
		return nil, err
	}
	return &SparseEncoder{tokenizer: tokenizer, stats: stats}, nil
}

func (e *SparseEncoder) EncodeChunk(chunk Chunk) (SparseVector, error) {
	if e == nil || e.tokenizer == nil {
		return SparseVector{}, fmt.Errorf("sparse encoder is not initialized")
	}
	values := map[uint32]float64{}
	fields := []struct {
		text    string
		weight  float64
		average float64
	}{
		{text: chunk.Title, weight: sparseTitleWeight, average: e.stats.AverageTitle},
		{text: chunk.HeadingPath, weight: sparseHeadingWeight, average: e.stats.AverageHeading},
		{text: chunk.SourceRef, weight: sparseSourceWeight, average: e.stats.AverageSource},
		{text: chunk.Content, weight: sparseBodyWeight, average: e.stats.AverageBody},
	}
	for fieldIndex, field := range fields {
		ids, err := e.tokenizer.TokenIDs(field.text)
		if err != nil {
			return SparseVector{}, fmt.Errorf("tokenize sparse field %d: %w", fieldIndex, err)
		}
		if err := addBM25Field(values, ids, field.weight, field.average); err != nil {
			return SparseVector{}, fmt.Errorf("encode sparse field %d: %w", fieldIndex, err)
		}
	}
	return sparseMapToVector(values)
}

func (e *SparseEncoder) EncodeQuery(text string) (SparseVector, error) {
	if e == nil || e.tokenizer == nil {
		return SparseVector{}, fmt.Errorf("sparse encoder is not initialized")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return SparseVector{}, fmt.Errorf("sparse query is required")
	}
	ids, err := e.tokenizer.TokenIDs(text)
	if err != nil {
		return SparseVector{}, fmt.Errorf("tokenize sparse query: %w", err)
	}
	if err := validateTokenIDs(ids); err != nil {
		return SparseVector{}, err
	}
	counts := tokenCounts(ids)
	values := make(map[uint32]float64, len(counts))
	for id, count := range counts {
		tf := float64(count)
		values[id] = tf * (sparseK1 + 1) / (tf + sparseK1)
	}
	return sparseMapToVector(values)
}

func addBM25Field(output map[uint32]float64, ids []int, weight, averageLength float64) error {
	if err := validateTokenIDs(ids); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	counts := tokenCounts(ids)
	lengthRatio := float64(len(ids)) / averageLength
	normalizer := sparseK1 * (1 - sparseB + sparseB*lengthRatio)
	for id, count := range counts {
		tf := float64(count)
		output[id] += weight * tf * (sparseK1 + 1) / (tf + normalizer)
	}
	return nil
}

func tokenCounts(ids []int) map[uint32]int {
	counts := make(map[uint32]int, len(ids))
	for _, id := range ids {
		counts[uint32(id)]++
	}
	return counts
}

func validateTokenIDs(ids []int) error {
	for index, id := range ids {
		if id < 0 {
			return fmt.Errorf("token ID %d at position %d must be non-negative", id, index)
		}
	}
	return nil
}

func sparseMapToVector(values map[uint32]float64) (SparseVector, error) {
	indices := make([]uint32, 0, len(values))
	for index, value := range values {
		if value == 0 {
			continue
		}
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return SparseVector{}, fmt.Errorf("sparse value for index %d must be finite and positive", index)
		}
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	vector := SparseVector{Indices: indices, Values: make([]float32, len(indices))}
	for i, index := range indices {
		vector.Values[i] = float32(values[index])
	}
	if err := validateSparseVector(vector); err != nil {
		return SparseVector{}, err
	}
	return vector, nil
}

func validateSparseVector(vector SparseVector) error {
	if len(vector.Indices) == 0 || len(vector.Indices) != len(vector.Values) {
		return fmt.Errorf("sparse indices and values must be non-empty with equal length")
	}
	for i, value := range vector.Values {
		if value <= 0 || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("sparse value %d must be finite and positive", i)
		}
		if i > 0 && vector.Indices[i] <= vector.Indices[i-1] {
			return fmt.Errorf("sparse indices must be strictly increasing")
		}
	}
	return nil
}

func (s *FieldStats) ensureNonZero() {
	if s.AverageTitle == 0 {
		s.AverageTitle = 1
	}
	if s.AverageHeading == 0 {
		s.AverageHeading = 1
	}
	if s.AverageSource == 0 {
		s.AverageSource = 1
	}
	if s.AverageBody == 0 {
		s.AverageBody = 1
	}
}

func (s FieldStats) validate() error {
	values := []float64{s.AverageTitle, s.AverageHeading, s.AverageSource, s.AverageBody}
	for i, value := range values {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("sparse average field length %d must be finite and positive", i)
		}
	}
	return nil
}
