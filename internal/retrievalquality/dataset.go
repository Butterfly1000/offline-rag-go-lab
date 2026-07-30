package retrievalquality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Split string

const (
	SplitTrain      Split = "train"
	SplitValidation Split = "validation"
)

type QueryKind string

const (
	QueryExact    QueryKind = "exact"
	QueryCode     QueryKind = "code"
	QuerySemantic QueryKind = "semantic"
	QueryMixed    QueryKind = "mixed"
	QueryNegative QueryKind = "negative"
)

type DatasetManifest struct {
	DatasetID       string `json:"dataset_id"`
	Version         string `json:"version"`
	CorpusSHA256    string `json:"corpus_sha256"`
	CasesSHA256     string `json:"cases_sha256"`
	TrainCount      int    `json:"train_count"`
	ValidationCount int    `json:"validation_count"`
}

type Chunk struct {
	KnowledgeScope string `json:"knowledge_scope"`
	DocumentID     string `json:"document_id"`
	ChunkID        string `json:"chunk_id"`
	Title          string `json:"title"`
	HeadingPath    string `json:"heading_path"`
	SourceRef      string `json:"source_ref"`
	StructureKind  string `json:"structure_kind"`
	Content        string `json:"content"`
	ContentHash    string `json:"content_hash"`
}

type Judgment struct {
	ChunkID   string `json:"chunk_id"`
	Relevance int    `json:"relevance"`
}

type GoldenCase struct {
	CaseID            string     `json:"case_id"`
	Query             string     `json:"query"`
	KnowledgeScope    string     `json:"knowledge_scope"`
	Kind              QueryKind  `json:"kind"`
	Split             Split      `json:"split"`
	Judgments         []Judgment `json:"judgments"`
	ForbiddenChunkIDs []string   `json:"forbidden_chunk_ids"`
	Notes             string     `json:"notes,omitempty"`
}

type Dataset struct {
	Manifest DatasetManifest
	Corpus   []Chunk
	Cases    []GoldenCase
}

func LoadDataset(dir string) (Dataset, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Dataset{}, fmt.Errorf("dataset directory is required")
	}
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset manifest: %w", err)
	}
	corpusBytes, err := os.ReadFile(filepath.Join(dir, "corpus.json"))
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset corpus: %w", err)
	}
	caseBytes, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		return Dataset{}, fmt.Errorf("read dataset cases: %w", err)
	}
	var dataset Dataset
	if err := decodeStrictJSON(manifestBytes, &dataset.Manifest); err != nil {
		return Dataset{}, fmt.Errorf("decode dataset manifest: %w", err)
	}
	if got := sha256Hex(corpusBytes); got != strings.ToLower(strings.TrimSpace(dataset.Manifest.CorpusSHA256)) {
		return Dataset{}, fmt.Errorf("corpus_sha256 mismatch: got %s want %s", got, dataset.Manifest.CorpusSHA256)
	}
	if got := sha256Hex(caseBytes); got != strings.ToLower(strings.TrimSpace(dataset.Manifest.CasesSHA256)) {
		return Dataset{}, fmt.Errorf("cases_sha256 mismatch: got %s want %s", got, dataset.Manifest.CasesSHA256)
	}
	if err := decodeStrictJSON(corpusBytes, &dataset.Corpus); err != nil {
		return Dataset{}, fmt.Errorf("decode dataset corpus: %w", err)
	}
	if err := decodeStrictJSON(caseBytes, &dataset.Cases); err != nil {
		return Dataset{}, fmt.Errorf("decode dataset cases: %w", err)
	}
	if err := dataset.Validate(); err != nil {
		return Dataset{}, err
	}
	return dataset, nil
}

func (d Dataset) CountSplit(split Split) int {
	count := 0
	for _, item := range d.Cases {
		if item.Split == split {
			count++
		}
	}
	return count
}

func (d Dataset) Validate() error {
	if strings.TrimSpace(d.Manifest.DatasetID) == "" || strings.TrimSpace(d.Manifest.Version) == "" {
		return fmt.Errorf("dataset_id and version are required")
	}
	if d.Manifest.TrainCount != 24 || d.Manifest.ValidationCount != 16 || len(d.Cases) != 40 {
		return fmt.Errorf("v1 requires 40 cases with split counts train=24 validation=16")
	}
	if d.CountSplit(SplitTrain) != d.Manifest.TrainCount || d.CountSplit(SplitValidation) != d.Manifest.ValidationCount {
		return fmt.Errorf("split counts do not match manifest: train=%d validation=%d", d.CountSplit(SplitTrain), d.CountSplit(SplitValidation))
	}
	chunks := make(map[string]Chunk, len(d.Corpus))
	scopes := map[string]bool{}
	for index, chunk := range d.Corpus {
		chunk = normalizeChunk(chunk)
		if chunk.KnowledgeScope == "" || chunk.DocumentID == "" || chunk.ChunkID == "" ||
			chunk.SourceRef == "" || chunk.StructureKind == "" || chunk.Content == "" || chunk.ContentHash == "" {
			return fmt.Errorf("corpus chunk %d is missing required identity or content", index)
		}
		if _, exists := chunks[chunk.ChunkID]; exists {
			return fmt.Errorf("duplicate chunk_id %q", chunk.ChunkID)
		}
		if got := sha256Hex([]byte(chunk.Content)); got != chunk.ContentHash {
			return fmt.Errorf("chunk %q content_hash mismatch: got %s want %s", chunk.ChunkID, got, chunk.ContentHash)
		}
		chunks[chunk.ChunkID] = chunk
		scopes[chunk.KnowledgeScope] = true
	}
	if len(chunks) < 20 {
		return fmt.Errorf("v1 requires at least 20 corpus chunks, got %d", len(chunks))
	}
	if len(scopes) != 2 {
		return fmt.Errorf("v1 requires exactly two knowledge scopes, got %d", len(scopes))
	}
	caseIDs := map[string]bool{}
	kinds := map[QueryKind]bool{}
	for index, item := range d.Cases {
		item.CaseID = strings.TrimSpace(item.CaseID)
		item.Query = strings.TrimSpace(item.Query)
		item.KnowledgeScope = strings.TrimSpace(item.KnowledgeScope)
		if item.CaseID == "" || item.Query == "" || item.KnowledgeScope == "" {
			return fmt.Errorf("golden case %d is missing case_id, query, or knowledge_scope", index)
		}
		if caseIDs[item.CaseID] {
			return fmt.Errorf("duplicate case_id %q", item.CaseID)
		}
		caseIDs[item.CaseID] = true
		if item.Split != SplitTrain && item.Split != SplitValidation {
			return fmt.Errorf("case %q has invalid split %q", item.CaseID, item.Split)
		}
		if !validQueryKind(item.Kind) {
			return fmt.Errorf("case %q has invalid query kind %q", item.CaseID, item.Kind)
		}
		kinds[item.Kind] = true
		if item.Kind == QueryNegative && len(item.Judgments) != 0 {
			return fmt.Errorf("negative case %q cannot have judgments", item.CaseID)
		}
		if item.Kind != QueryNegative && (len(item.Judgments) < 1 || len(item.Judgments) > 5) {
			return fmt.Errorf("case %q requires 1-5 judgments", item.CaseID)
		}
		judged := map[string]bool{}
		for _, judgment := range item.Judgments {
			id := strings.TrimSpace(judgment.ChunkID)
			chunk, exists := chunks[id]
			if !exists {
				return fmt.Errorf("case %q judgment references unknown chunk %q", item.CaseID, id)
			}
			if chunk.KnowledgeScope != item.KnowledgeScope {
				return fmt.Errorf("case %q judgment chunk %q belongs to knowledge_scope %q, want %q", item.CaseID, id, chunk.KnowledgeScope, item.KnowledgeScope)
			}
			if judgment.Relevance < 1 || judgment.Relevance > 3 {
				return fmt.Errorf("case %q judgment %q relevance=%d, want 1-3", item.CaseID, id, judgment.Relevance)
			}
			if judged[id] {
				return fmt.Errorf("case %q has duplicate judgment %q", item.CaseID, id)
			}
			judged[id] = true
		}
		if len(item.ForbiddenChunkIDs) == 0 {
			return fmt.Errorf("case %q requires at least one forbidden chunk", item.CaseID)
		}
		forbidden := map[string]bool{}
		for _, rawID := range item.ForbiddenChunkIDs {
			id := strings.TrimSpace(rawID)
			chunk, exists := chunks[id]
			if !exists {
				return fmt.Errorf("case %q forbidden references unknown chunk %q", item.CaseID, id)
			}
			if forbidden[id] {
				return fmt.Errorf("case %q has duplicate forbidden chunk %q", item.CaseID, id)
			}
			if judged[id] {
				return fmt.Errorf("case %q chunk %q is both judged and forbidden", item.CaseID, id)
			}
			if chunk.KnowledgeScope == item.KnowledgeScope {
				return fmt.Errorf(
					"case %q forbidden chunk must belong to another knowledge_scope: %q",
					item.CaseID, id,
				)
			}
			forbidden[id] = true
		}
	}
	for _, kind := range []QueryKind{QueryExact, QueryCode, QuerySemantic, QueryMixed, QueryNegative} {
		if !kinds[kind] {
			return fmt.Errorf("v1 does not cover query kind %q", kind)
		}
	}
	return nil
}

func normalizeChunk(chunk Chunk) Chunk {
	chunk.KnowledgeScope = strings.TrimSpace(chunk.KnowledgeScope)
	chunk.DocumentID = strings.TrimSpace(chunk.DocumentID)
	chunk.ChunkID = strings.TrimSpace(chunk.ChunkID)
	chunk.Title = strings.TrimSpace(chunk.Title)
	chunk.HeadingPath = strings.TrimSpace(chunk.HeadingPath)
	chunk.SourceRef = strings.TrimSpace(chunk.SourceRef)
	chunk.StructureKind = strings.TrimSpace(chunk.StructureKind)
	chunk.ContentHash = strings.ToLower(strings.TrimSpace(chunk.ContentHash))
	return chunk
}

func validQueryKind(kind QueryKind) bool {
	switch kind {
	case QueryExact, QueryCode, QuerySemantic, QueryMixed, QueryNegative:
		return true
	default:
		return false
	}
}

func decodeStrictJSON(content []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("unexpected trailing JSON value")
	}
	return nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func sortedCaseCopy(cases []GoldenCase) []GoldenCase {
	result := append([]GoldenCase(nil), cases...)
	sort.Slice(result, func(i, j int) bool { return result[i].CaseID < result[j].CaseID })
	return result
}
