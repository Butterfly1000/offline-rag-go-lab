package retrievalquality

import (
	"context"
	"math"
	"reflect"
	"testing"
)

func TestFitIsotonicPoolsDecreasingBlocksAndPredictsMonotonically(t *testing.T) {
	calibrator, err := FitIsotonic([]CalibrationSample{
		{Score: 1, Label: 0},
		{Score: 2, Label: 1},
		{Score: 3, Label: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []float64{0, 0.5}; !reflect.DeepEqual(calibrator.Values, want) {
		t.Fatalf("values=%v, want %v", calibrator.Values, want)
	}
	if want := []float64{1, 3}; !reflect.DeepEqual(calibrator.Breakpoints, want) {
		t.Fatalf("breakpoints=%v, want %v", calibrator.Breakpoints, want)
	}
	previous := -1.0
	for _, score := range []float64{-100, 1, 1.5, 2, 3, 100} {
		probability := calibrator.Predict(score)
		if probability < previous || probability < 0 || probability > 1 {
			t.Fatalf("score=%v probability=%v previous=%v", score, probability, previous)
		}
		previous = probability
	}
}

func TestFitIsotonicAggregatesRepeatedScoresDeterministically(t *testing.T) {
	left, err := FitIsotonic([]CalibrationSample{
		{Score: 2, Label: 1},
		{Score: 1, Label: 0},
		{Score: 2, Label: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	right, err := FitIsotonic([]CalibrationSample{
		{Score: 2, Label: 0},
		{Score: 2, Label: 1},
		{Score: 1, Label: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("left=%+v right=%+v", left, right)
	}
	if want := []float64{0, 0.5}; !reflect.DeepEqual(left.Values, want) {
		t.Fatalf("values=%v, want %v", left.Values, want)
	}
}

func TestFitIsotonicRejectsEmptyNonFiniteOrInvalidLabels(t *testing.T) {
	for _, samples := range [][]CalibrationSample{
		nil,
		{{Score: math.NaN(), Label: 0}},
		{{Score: math.Inf(1), Label: 0}},
		{{Score: 1, Label: math.NaN()}},
		{{Score: 1, Label: -0.1}},
		{{Score: 1, Label: 1.1}},
	} {
		if _, err := FitIsotonic(samples); err == nil {
			t.Fatalf("samples=%v should fail", samples)
		}
	}
}

func TestBrierScoreAndECE(t *testing.T) {
	calibrator := Calibrator{
		Breakpoints: []float64{0.5, 1},
		Values:      []float64{0.25, 0.75},
	}
	samples := []CalibrationSample{
		{Score: 0.1, Label: 0},
		{Score: 0.9, Label: 1},
	}
	brier, err := BrierScore(samples, calibrator)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(brier-0.0625) > 1e-12 {
		t.Fatalf("brier=%v, want 0.0625", brier)
	}
	ece, err := ExpectedCalibrationError(samples, calibrator, 2)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(ece-0.25) > 1e-12 {
		t.Fatalf("ece=%v, want 0.25", ece)
	}
}

type calibrationCorpusStrategy struct {
	corpus []Chunk
}

func (calibrationCorpusStrategy) Name() string { return "calibration_test" }

func (s calibrationCorpusStrategy) Search(_ context.Context, query Query) (SearchResult, error) {
	result := SearchResult{}
	for index, chunk := range s.corpus {
		if chunk.KnowledgeScope != query.KnowledgeScope {
			continue
		}
		result.Candidates = append(result.Candidates, Candidate{
			KnowledgeScope: chunk.KnowledgeScope, ChunkID: chunk.ChunkID,
			Score: float64(len(s.corpus) - index),
		})
		if len(result.Candidates) == query.Limit {
			break
		}
	}
	return result, nil
}

func TestCollectCalibrationSamplesUsesOnlyRequestedSplitAndLabelsCandidates(t *testing.T) {
	dataset, err := LoadDataset("testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	samples, err := CollectCalibrationSamples(
		context.Background(), dataset, SplitTrain,
		calibrationCorpusStrategy{corpus: dataset.Corpus}, 10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != dataset.Manifest.TrainCount*10 {
		t.Fatalf("samples=%d, want %d", len(samples), dataset.Manifest.TrainCount*10)
	}
	var positives, negatives int
	for _, sample := range samples {
		if sample.Label == 1 {
			positives++
		} else if sample.Label == 0 {
			negatives++
		}
	}
	if positives == 0 || negatives == 0 {
		t.Fatalf("positives=%d negatives=%d", positives, negatives)
	}
}
