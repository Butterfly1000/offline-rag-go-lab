package retrievalquality

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

type CalibrationSample struct {
	Score float64 `json:"score"`
	Label float64 `json:"label"`
}

type Calibrator struct {
	Breakpoints []float64 `json:"breakpoints"`
	Values      []float64 `json:"values"`
}

func FitIsotonic(samples []CalibrationSample) (Calibrator, error) {
	if len(samples) == 0 {
		return Calibrator{}, fmt.Errorf("calibration samples are required")
	}
	ordered := append([]CalibrationSample(nil), samples...)
	for index, sample := range ordered {
		if !finite(sample.Score) || !finite(sample.Label) || sample.Label < 0 || sample.Label > 1 {
			return Calibrator{}, fmt.Errorf("calibration sample %d requires finite score and label in [0,1]", index)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Score < ordered[j].Score })
	type block struct {
		maxScore float64
		sum      float64
		weight   int
	}
	var grouped []block
	for _, sample := range ordered {
		if len(grouped) > 0 && grouped[len(grouped)-1].maxScore == sample.Score {
			grouped[len(grouped)-1].sum += sample.Label
			grouped[len(grouped)-1].weight++
		} else {
			grouped = append(grouped, block{maxScore: sample.Score, sum: sample.Label, weight: 1})
		}
	}
	blocks := make([]block, 0, len(grouped))
	for _, current := range grouped {
		blocks = append(blocks, current)
		for len(blocks) >= 2 {
			left := blocks[len(blocks)-2]
			right := blocks[len(blocks)-1]
			if left.sum/float64(left.weight) <= right.sum/float64(right.weight) {
				break
			}
			blocks[len(blocks)-2] = block{
				maxScore: right.maxScore,
				sum:      left.sum + right.sum,
				weight:   left.weight + right.weight,
			}
			blocks = blocks[:len(blocks)-1]
		}
	}
	result := Calibrator{
		Breakpoints: make([]float64, len(blocks)),
		Values:      make([]float64, len(blocks)),
	}
	for index, item := range blocks {
		result.Breakpoints[index] = item.maxScore
		result.Values[index] = item.sum / float64(item.weight)
	}
	return result, nil
}

func (c Calibrator) Predict(score float64) float64 {
	if len(c.Breakpoints) == 0 || len(c.Breakpoints) != len(c.Values) || math.IsNaN(score) {
		return 0
	}
	index := sort.Search(len(c.Breakpoints), func(index int) bool {
		return score <= c.Breakpoints[index]
	})
	if index == len(c.Breakpoints) {
		index--
	}
	value := c.Values[index]
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func BrierScore(samples []CalibrationSample, calibrator Calibrator) (float64, error) {
	if err := validateCalibrationInputs(samples, calibrator); err != nil {
		return 0, err
	}
	var total float64
	for _, sample := range samples {
		difference := calibrator.Predict(sample.Score) - sample.Label
		total += difference * difference
	}
	return total / float64(len(samples)), nil
}

func ExpectedCalibrationError(samples []CalibrationSample, calibrator Calibrator, bins int) (float64, error) {
	if bins <= 0 {
		return 0, fmt.Errorf("calibration bins must be positive")
	}
	if err := validateCalibrationInputs(samples, calibrator); err != nil {
		return 0, err
	}
	type bucket struct {
		count       int
		probability float64
		label       float64
	}
	buckets := make([]bucket, bins)
	for _, sample := range samples {
		probability := calibrator.Predict(sample.Score)
		index := int(probability * float64(bins))
		if index == bins {
			index--
		}
		buckets[index].count++
		buckets[index].probability += probability
		buckets[index].label += sample.Label
	}
	var result float64
	for _, item := range buckets {
		if item.count == 0 {
			continue
		}
		confidence := item.probability / float64(item.count)
		accuracy := item.label / float64(item.count)
		result += float64(item.count) / float64(len(samples)) * math.Abs(confidence-accuracy)
	}
	return result, nil
}

func CollectCalibrationSamples(
	ctx context.Context,
	dataset Dataset,
	split Split,
	strategy Strategy,
	k int,
) ([]CalibrationSample, error) {
	if split != SplitTrain && split != SplitValidation {
		return nil, fmt.Errorf("calibration split %q is invalid", split)
	}
	if strategy == nil || strings.TrimSpace(strategy.Name()) == "" || k <= 0 {
		return nil, fmt.Errorf("calibration strategy and positive candidate limit are required")
	}
	corpusScopes := make(map[string]string, len(dataset.Corpus))
	for _, chunk := range dataset.Corpus {
		corpusScopes[chunk.ChunkID] = chunk.KnowledgeScope
	}
	var samples []CalibrationSample
	for _, golden := range sortedCaseCopy(dataset.Cases) {
		if golden.Split != split {
			continue
		}
		result, err := strategy.Search(ctx, Query{
			CaseID: golden.CaseID, Text: golden.Query,
			KnowledgeScope: golden.KnowledgeScope, Kind: golden.Kind, Limit: k,
		})
		if err != nil {
			return nil, fmt.Errorf("collect calibration case %s: %w", golden.CaseID, err)
		}
		if err := validateCandidateLeg("calibration "+golden.CaseID, result.Candidates); err != nil {
			return nil, err
		}
		judged := make(map[string]bool, len(golden.Judgments))
		for _, judgment := range golden.Judgments {
			if judgment.Relevance > 0 {
				judged[judgment.ChunkID] = true
			}
		}
		for _, candidate := range result.Candidates {
			scope, exists := corpusScopes[candidate.ChunkID]
			if !exists {
				return nil, fmt.Errorf("calibration case %s returned unknown chunk %q", golden.CaseID, candidate.ChunkID)
			}
			if candidate.KnowledgeScope != golden.KnowledgeScope || scope != golden.KnowledgeScope {
				return nil, fmt.Errorf("calibration case %s returned cross-scope chunk %q", golden.CaseID, candidate.ChunkID)
			}
			label := 0.0
			if judged[candidate.ChunkID] {
				label = 1
			}
			samples = append(samples, CalibrationSample{Score: candidate.Score, Label: label})
		}
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("calibration split %q produced no samples", split)
	}
	return samples, nil
}

func validateCalibrationInputs(samples []CalibrationSample, calibrator Calibrator) error {
	if len(samples) == 0 {
		return fmt.Errorf("calibration samples are required")
	}
	if len(calibrator.Breakpoints) == 0 || len(calibrator.Breakpoints) != len(calibrator.Values) {
		return fmt.Errorf("calibrator breakpoints and values are required and must align")
	}
	for index, sample := range samples {
		if !finite(sample.Score) || !finite(sample.Label) || sample.Label < 0 || sample.Label > 1 {
			return fmt.Errorf("calibration sample %d is invalid", index)
		}
	}
	for index := range calibrator.Breakpoints {
		if !finite(calibrator.Breakpoints[index]) || !finite(calibrator.Values[index]) ||
			calibrator.Values[index] < 0 || calibrator.Values[index] > 1 {
			return fmt.Errorf("calibrator point %d is invalid", index)
		}
		if index > 0 && (calibrator.Breakpoints[index] <= calibrator.Breakpoints[index-1] ||
			calibrator.Values[index] < calibrator.Values[index-1]) {
			return fmt.Errorf("calibrator points must be strictly score-ordered and monotonic")
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
