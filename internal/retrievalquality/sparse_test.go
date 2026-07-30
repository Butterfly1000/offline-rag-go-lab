package retrievalquality

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

type tokenMap map[string][]int

func (m tokenMap) TokenIDs(text string) ([]int, error) {
	if strings.HasPrefix(text, "error:") {
		return nil, errors.New("tokenizer failed")
	}
	return append([]int(nil), m[text]...), nil
}

func TestSparseEncoderAppliesFieldWeightsAndSortsIndices(t *testing.T) {
	encoder, err := NewSparseEncoder(tokenMap{
		"title": {30}, "heading": {20}, "source": {40}, "body": {10},
	}, FieldStats{AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 1})
	if err != nil {
		t.Fatal(err)
	}
	vector, err := encoder.EncodeChunk(Chunk{
		Title: "title", HeadingPath: "heading", SourceRef: "source", Content: "body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{10, 20, 30, 40}; !reflect.DeepEqual(vector.Indices, want) {
		t.Fatalf("indices=%v, want %v", vector.Indices, want)
	}
	wantValues := []float32{1, 2, 3, 1.5}
	for i, want := range wantValues {
		if math.Abs(float64(vector.Values[i]-want)) > 1e-6 {
			t.Fatalf("value[%d]=%v, want %v", i, vector.Values[i], want)
		}
	}
}

func TestSparseEncoderUsesBM25SaturationAndLengthNormalization(t *testing.T) {
	encoder, err := NewSparseEncoder(tokenMap{
		"repeated": {7, 7},
		"long":     {8, 9},
	}, FieldStats{AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 2})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := encoder.EncodeChunk(Chunk{Content: "repeated"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := repeated.Values[0], float32(1.375); math.Abs(float64(got-want)) > 1e-6 {
		t.Fatalf("repeated BM25=%v, want %v", got, want)
	}

	longEncoder, err := NewSparseEncoder(tokenMap{"long": {8, 9}}, FieldStats{
		AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	long, err := longEncoder.EncodeChunk(Chunk{Content: "long"})
	if err != nil {
		t.Fatal(err)
	}
	if long.Values[0] >= 1 {
		t.Fatalf("long-field normalized value=%v, want less than 1", long.Values[0])
	}
}

func TestSparseEncoderMergesDuplicateTermsAcrossFields(t *testing.T) {
	encoder, err := NewSparseEncoder(tokenMap{
		"title": {5}, "body": {5},
	}, FieldStats{AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 1})
	if err != nil {
		t.Fatal(err)
	}
	vector, err := encoder.EncodeChunk(Chunk{Title: "title", Content: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vector.Indices, []uint32{5}) || len(vector.Values) != 1 || math.Abs(float64(vector.Values[0]-4)) > 1e-6 {
		t.Fatalf("merged vector=%+v, want index 5 value 4", vector)
	}
}

func TestSparseEncoderRejectsInvalidTokenIDsAndStats(t *testing.T) {
	if _, err := NewSparseEncoder(tokenMap{}, FieldStats{}); err == nil {
		t.Fatal("expected invalid stats error")
	}
	encoder, err := NewSparseEncoder(tokenMap{"bad": {-1}}, FieldStats{
		AverageTitle: 1, AverageHeading: 1, AverageSource: 1, AverageBody: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encoder.EncodeQuery("bad"); err == nil || !strings.Contains(err.Error(), "token ID") {
		t.Fatalf("error=%v, want token ID failure", err)
	}
}

func TestBuildFieldStatsUsesActualTokenLengths(t *testing.T) {
	stats, err := BuildFieldStats([]Chunk{
		{Title: "t1", HeadingPath: "h1", SourceRef: "s1", Content: "b1"},
		{Title: "t2", HeadingPath: "h2", SourceRef: "s2", Content: "b2"},
	}, tokenMap{
		"t1": {1}, "t2": {1, 2, 3},
		"h1": {1, 2}, "h2": {3, 4},
		"s1": {1}, "s2": {2},
		"b1": {1, 2, 3}, "b2": {4},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := FieldStats{AverageTitle: 2, AverageHeading: 2, AverageSource: 1, AverageBody: 2}
	if stats != want {
		t.Fatalf("stats=%+v, want %+v", stats, want)
	}
}
