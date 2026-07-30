package retrievalquality

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type FailureStage string

const (
	FailureDataset      FailureStage = "dataset"
	FailureDenseRecall  FailureStage = "dense_recall"
	FailureSparseRecall FailureStage = "sparse_recall"
	FailureFusion       FailureStage = "fusion"
	FailureRerank       FailureStage = "rerank"
	FailureDiversity    FailureStage = "diversity"
	FailureIsolation    FailureStage = "isolation"
	FailurePolicy       FailureStage = "policy"
)

type Query struct {
	CaseID         string
	Text           string
	KnowledgeScope string
	Kind           QueryKind
	Limit          int
}

type Candidate struct {
	KnowledgeScope string             `json:"knowledge_scope"`
	DocumentID     string             `json:"document_id,omitempty"`
	ChunkID        string             `json:"chunk_id"`
	HeadingPath    string             `json:"heading_path,omitempty"`
	Content        string             `json:"content,omitempty"`
	Score          float64            `json:"score"`
	Trace          map[string]float64 `json:"trace,omitempty"`
	Reason         string             `json:"reason,omitempty"`
}

type SearchResult struct {
	Candidates []Candidate   `json:"candidates"`
	Abstained  bool          `json:"abstained"`
	Duration   time.Duration `json:"-"`
	Warnings   []string      `json:"warnings,omitempty"`
}

type Strategy interface {
	Name() string
	Search(context.Context, Query) (SearchResult, error)
}

type CaseResult struct {
	CaseID            string        `json:"case_id"`
	Kind              QueryKind     `json:"kind"`
	RecallAt3         float64       `json:"recall_at_3"`
	RecallAt10        float64       `json:"recall_at_10"`
	MRRAt10           float64       `json:"mrr_at_10"`
	NDCGAt10          float64       `json:"ndcg_at_10"`
	NegativePassed    bool          `json:"negative_passed"`
	ScopeIsolated     bool          `json:"scope_isolated"`
	ForbiddenHits     []string      `json:"forbidden_hits"`
	RetrievedChunkIDs []string      `json:"retrieved_chunk_ids"`
	Latency           time.Duration `json:"latency_ns"`
	Warnings          []string      `json:"warnings,omitempty"`
}

type KindReport struct {
	CaseCount        int     `json:"case_count"`
	MeanRecallAt10   float64 `json:"mean_recall_at_10"`
	MeanNDCGAt10     float64 `json:"mean_ndcg_at_10"`
	NegativePassRate float64 `json:"negative_pass_rate"`
}

type Report struct {
	DatasetID         string                   `json:"dataset_id"`
	DatasetVersion    string                   `json:"dataset_version"`
	Strategy          string                   `json:"strategy"`
	Split             Split                    `json:"split"`
	CaseCount         int                      `json:"case_count"`
	MeanRecallAt3     float64                  `json:"mean_recall_at_3"`
	MeanRecallAt10    float64                  `json:"mean_recall_at_10"`
	MeanMRRAt10       float64                  `json:"mean_mrr_at_10"`
	MeanNDCGAt10      float64                  `json:"mean_ndcg_at_10"`
	NegativePassRate  float64                  `json:"negative_pass_rate"`
	ScopeIsolation    float64                  `json:"scope_isolation"`
	ForbiddenHitCount int                      `json:"forbidden_hit_count"`
	LatencyP50        time.Duration            `json:"latency_p50_ns"`
	LatencyP95        time.Duration            `json:"latency_p95_ns"`
	FailureStage      FailureStage             `json:"failure_stage,omitempty"`
	Passed            bool                     `json:"passed"`
	ByKind            map[QueryKind]KindReport `json:"by_kind"`
	Cases             []CaseResult             `json:"cases"`
}

func Evaluate(ctx context.Context, dataset Dataset, split Split, strategy Strategy, k int) (Report, error) {
	report := Report{
		DatasetID: dataset.Manifest.DatasetID, DatasetVersion: dataset.Manifest.Version,
		Split: split, ByKind: map[QueryKind]KindReport{}, Cases: []CaseResult{},
	}
	if strategy == nil || strings.TrimSpace(strategy.Name()) == "" {
		report.FailureStage = FailureDataset
		return report, fmt.Errorf("evaluation strategy and name are required")
	}
	report.Strategy = strategy.Name()
	if split != SplitTrain && split != SplitValidation {
		report.FailureStage = FailureDataset
		return report, fmt.Errorf("evaluation split %q is invalid", split)
	}
	if k < 10 {
		report.FailureStage = FailureDataset
		return report, fmt.Errorf("evaluation k must be at least 10")
	}
	cases := sortedCaseCopy(dataset.Cases)
	corpusScopes := make(map[string]string, len(dataset.Corpus))
	for _, chunk := range dataset.Corpus {
		corpusScopes[strings.TrimSpace(chunk.ChunkID)] = strings.TrimSpace(chunk.KnowledgeScope)
	}
	var positiveCount, negativeCount, negativePasses, isolated int
	var latencies []time.Duration
	kindPositive := map[QueryKind]int{}
	kindNegative := map[QueryKind]int{}
	kindNegativePasses := map[QueryKind]int{}
	for _, golden := range cases {
		if golden.Split != split {
			continue
		}
		result, err := strategy.Search(ctx, Query{
			CaseID: golden.CaseID, Text: golden.Query, KnowledgeScope: golden.KnowledgeScope,
			Kind: golden.Kind, Limit: k,
		})
		if err != nil {
			report.FailureStage = stageForStrategy(strategy.Name())
			return report, fmt.Errorf("%s search case %s: %w", strategy.Name(), golden.CaseID, err)
		}
		if result.Duration < 0 {
			report.FailureStage = stageForStrategy(strategy.Name())
			return report, fmt.Errorf("case %s returned negative duration", golden.CaseID)
		}
		if len(result.Candidates) > k {
			report.FailureStage = stageForStrategy(strategy.Name())
			return report, fmt.Errorf("case %s returned %d candidates, limit %d", golden.CaseID, len(result.Candidates), k)
		}
		caseResult := CaseResult{
			CaseID: golden.CaseID, Kind: golden.Kind, ScopeIsolated: true,
			ForbiddenHits: []string{}, RetrievedChunkIDs: []string{},
			Latency: result.Duration, Warnings: append([]string(nil), result.Warnings...),
		}
		judgments, forbidden, seen := map[string]int{}, map[string]bool{}, map[string]bool{}
		for _, judgment := range golden.Judgments {
			judgments[judgment.ChunkID] = judgment.Relevance
		}
		for _, id := range golden.ForbiddenChunkIDs {
			forbidden[id] = true
		}
		for _, candidate := range result.Candidates {
			if strings.TrimSpace(candidate.KnowledgeScope) != golden.KnowledgeScope {
				report.FailureStage = FailureIsolation
				return report, fmt.Errorf("case %s returned cross-scope candidate %q", golden.CaseID, candidate.KnowledgeScope)
			}
			id := strings.TrimSpace(candidate.ChunkID)
			if id == "" || seen[id] {
				report.FailureStage = stageForStrategy(strategy.Name())
				return report, fmt.Errorf("case %s returned empty or duplicate chunk_id %q", golden.CaseID, id)
			}
			corpusScope, exists := corpusScopes[id]
			if !exists {
				report.FailureStage = stageForStrategy(strategy.Name())
				return report, fmt.Errorf("case %s returned unknown corpus chunk %q", golden.CaseID, id)
			}
			if corpusScope != golden.KnowledgeScope {
				report.FailureStage = FailureIsolation
				return report, fmt.Errorf("case %s corpus chunk %q belongs to knowledge_scope %q", golden.CaseID, id, corpusScope)
			}
			seen[id] = true
			caseResult.RetrievedChunkIDs = append(caseResult.RetrievedChunkIDs, id)
			if forbidden[id] {
				caseResult.ForbiddenHits = append(caseResult.ForbiddenHits, id)
			}
		}
		sort.Strings(caseResult.ForbiddenHits)
		report.ForbiddenHitCount += len(caseResult.ForbiddenHits)
		isolated++
		if golden.Kind == QueryNegative {
			negativeCount++
			kindNegative[golden.Kind]++
			caseResult.NegativePassed = negativePassed(result.Abstained, result.Candidates)
			if caseResult.NegativePassed {
				negativePasses++
				kindNegativePasses[golden.Kind]++
			}
		} else {
			positiveCount++
			kindPositive[golden.Kind]++
			caseResult.RecallAt3 = recallAt(caseResult.RetrievedChunkIDs, judgments, 3)
			caseResult.RecallAt10 = recallAt(caseResult.RetrievedChunkIDs, judgments, 10)
			caseResult.MRRAt10 = mrrAt(caseResult.RetrievedChunkIDs, judgments, 10)
			caseResult.NDCGAt10 = ndcgAt(caseResult.RetrievedChunkIDs, judgments, 10)
			report.MeanRecallAt3 += caseResult.RecallAt3
			report.MeanRecallAt10 += caseResult.RecallAt10
			report.MeanMRRAt10 += caseResult.MRRAt10
			report.MeanNDCGAt10 += caseResult.NDCGAt10
		}
		kind := report.ByKind[golden.Kind]
		kind.CaseCount++
		kind.MeanRecallAt10 += caseResult.RecallAt10
		kind.MeanNDCGAt10 += caseResult.NDCGAt10
		report.ByKind[golden.Kind] = kind
		latencies = append(latencies, result.Duration)
		report.Cases = append(report.Cases, caseResult)
	}
	report.CaseCount = len(report.Cases)
	if report.CaseCount == 0 {
		report.FailureStage = FailureDataset
		return report, fmt.Errorf("evaluation split %q contains no cases", split)
	}
	if positiveCount > 0 {
		report.MeanRecallAt3 /= float64(positiveCount)
		report.MeanRecallAt10 /= float64(positiveCount)
		report.MeanMRRAt10 /= float64(positiveCount)
		report.MeanNDCGAt10 /= float64(positiveCount)
	}
	if negativeCount > 0 {
		report.NegativePassRate = float64(negativePasses) / float64(negativeCount)
	}
	report.ScopeIsolation = float64(isolated) / float64(report.CaseCount)
	report.LatencyP50 = percentile(latencies, 0.50)
	report.LatencyP95 = percentile(latencies, 0.95)
	for kind, item := range report.ByKind {
		if count := kindPositive[kind]; count > 0 {
			item.MeanRecallAt10 /= float64(count)
			item.MeanNDCGAt10 /= float64(count)
		}
		if count := kindNegative[kind]; count > 0 {
			item.NegativePassRate = float64(kindNegativePasses[kind]) / float64(count)
		}
		report.ByKind[kind] = item
	}
	report.Passed = report.ScopeIsolation == 1 && report.ForbiddenHitCount == 0
	return report, nil
}

func stageForStrategy(name string) FailureStage {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "sparse"):
		return FailureSparseRecall
	case strings.Contains(name, "rerank"):
		return FailureRerank
	case strings.Contains(name, "hybrid"), strings.Contains(name, "rrf"):
		return FailureFusion
	case strings.Contains(name, "policy"):
		return FailurePolicy
	default:
		return FailureDenseRecall
	}
}
