package retrievalquality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDatasetV1EnforcesReviewedShape(t *testing.T) {
	dataset, err := LoadDataset("testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(dataset.Corpus); got < 20 {
		t.Fatalf("corpus chunks=%d, want at least 20", got)
	}
	if got := len(dataset.Cases); got != 40 {
		t.Fatalf("cases=%d, want 40", got)
	}
	if got := dataset.CountSplit(SplitTrain); got != 24 {
		t.Fatalf("train=%d, want 24", got)
	}
	if got := dataset.CountSplit(SplitValidation); got != 16 {
		t.Fatalf("validation=%d, want 16", got)
	}
	kinds, scopes := map[QueryKind]bool{}, map[string]bool{}
	for _, item := range dataset.Cases {
		kinds[item.Kind] = true
		scopes[item.KnowledgeScope] = true
	}
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed, QueryNegative} {
		if !kinds[kind] {
			t.Fatalf("query kind %q is not covered", kind)
		}
	}
	if got := len(scopes); got != 2 {
		t.Fatalf("scopes=%d, want 2", got)
	}
}

func TestLoadDatasetRejectsChangedFileChecksum(t *testing.T) {
	dir := copyDatasetFixture(t)
	path := filepath.Join(dir, "cases.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDataset(dir); err == nil || !strings.Contains(err.Error(), "cases_sha256") {
		t.Fatalf("error=%v, want cases_sha256 mismatch", err)
	}
}

func TestDecodeStrictJSONRejectsTrailingValue(t *testing.T) {
	var manifest DatasetManifest
	err := decodeStrictJSON([]byte(`{"dataset_id":"a","version":"v1"} {}`), &manifest)
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("error=%v, want trailing JSON failure", err)
	}
}

func TestDatasetValidateRejectsBrokenContracts(t *testing.T) {
	base, err := LoadDataset("testdata/golden/v1")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Dataset)
		want   string
	}{
		{
			name: "duplicate case ID",
			mutate: func(dataset *Dataset) {
				dataset.Cases[1].CaseID = dataset.Cases[0].CaseID
			},
			want: "duplicate case_id",
		},
		{
			name: "unknown judged chunk",
			mutate: func(dataset *Dataset) {
				dataset.Cases[0].Judgments[0].ChunkID = "missing"
			},
			want: "unknown chunk",
		},
		{
			name: "cross scope judgment",
			mutate: func(dataset *Dataset) {
				for _, chunk := range dataset.Corpus {
					if chunk.KnowledgeScope != dataset.Cases[0].KnowledgeScope {
						dataset.Cases[0].Judgments[0].ChunkID = chunk.ChunkID
						return
					}
				}
			},
			want: "belongs to knowledge_scope",
		},
		{
			name: "overlapping forbidden",
			mutate: func(dataset *Dataset) {
				dataset.Cases[0].ForbiddenChunkIDs[0] = dataset.Cases[0].Judgments[0].ChunkID
			},
			want: "both judged and forbidden",
		},
		{
			name: "same scope forbidden",
			mutate: func(dataset *Dataset) {
				for _, chunk := range dataset.Corpus {
					if chunk.KnowledgeScope == dataset.Cases[0].KnowledgeScope &&
						chunk.ChunkID != dataset.Cases[0].Judgments[0].ChunkID {
						dataset.Cases[0].ForbiddenChunkIDs[0] = chunk.ChunkID
						return
					}
				}
			},
			want: "forbidden chunk must belong to another knowledge_scope",
		},
		{
			name: "invalid relevance",
			mutate: func(dataset *Dataset) {
				dataset.Cases[0].Judgments[0].Relevance = 4
			},
			want: "relevance",
		},
		{
			name: "wrong split count",
			mutate: func(dataset *Dataset) {
				for i := range dataset.Cases {
					if dataset.Cases[i].Split == SplitValidation {
						dataset.Cases[i].Split = SplitTrain
						return
					}
				}
			},
			want: "split counts",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataset := cloneDataset(base)
			tt.mutate(&dataset)
			if err := dataset.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want substring %q", err, tt.want)
			}
		})
	}
}

func copyDatasetFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"manifest.json", "corpus.json", "cases.json"} {
		content, err := os.ReadFile(filepath.Join("testdata/golden/v1", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cloneDataset(input Dataset) Dataset {
	output := input
	output.Corpus = append([]Chunk(nil), input.Corpus...)
	output.Cases = make([]GoldenCase, len(input.Cases))
	for i, item := range input.Cases {
		output.Cases[i] = item
		output.Cases[i].Judgments = append([]Judgment(nil), item.Judgments...)
		output.Cases[i].ForbiddenChunkIDs = append([]string(nil), item.ForbiddenChunkIDs...)
	}
	return output
}
