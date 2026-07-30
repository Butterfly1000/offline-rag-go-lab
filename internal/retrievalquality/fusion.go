package retrievalquality

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type FusionWeights struct {
	Dense  float64 `json:"dense"`
	Sparse float64 `json:"sparse"`
}

func FuseRRF(dense, sparse []Candidate, weights FusionWeights, rankConstant int) ([]Candidate, error) {
	if err := validateFusionWeights(weights, rankConstant); err != nil {
		return nil, err
	}
	if err := validateCandidateLeg("dense", dense); err != nil {
		return nil, err
	}
	if err := validateCandidateLeg("sparse", sparse); err != nil {
		return nil, err
	}
	type fusedItem struct {
		candidate Candidate
		bestRank  int
	}
	items := map[string]*fusedItem{}
	addLeg := func(name string, candidates []Candidate, weight float64) error {
		for index, candidate := range candidates {
			rank := index + 1
			item, exists := items[candidate.ChunkID]
			if !exists {
				copyCandidate := candidate
				copyCandidate.Score = 0
				copyCandidate.Trace = map[string]float64{}
				item = &fusedItem{candidate: copyCandidate, bestRank: rank}
				items[candidate.ChunkID] = item
			} else {
				if item.candidate.KnowledgeScope != candidate.KnowledgeScope {
					return fmt.Errorf("candidate %q scope conflict: %q vs %q", candidate.ChunkID, item.candidate.KnowledgeScope, candidate.KnowledgeScope)
				}
				if err := mergeCandidateMetadata(&item.candidate, candidate); err != nil {
					return err
				}
				if rank < item.bestRank {
					item.bestRank = rank
				}
			}
			contribution := weight / float64(rankConstant+rank)
			item.candidate.Trace[name+"_rank"] = float64(rank)
			item.candidate.Trace[name+"_score"] = candidate.Score
			item.candidate.Trace[name+"_contribution"] = contribution
			item.candidate.Score += contribution
		}
		return nil
	}
	if err := addLeg("dense", dense, weights.Dense); err != nil {
		return nil, err
	}
	if err := addLeg("sparse", sparse, weights.Sparse); err != nil {
		return nil, err
	}
	result := make([]fusedItem, 0, len(items))
	for _, item := range items {
		_, hasDense := item.candidate.Trace["dense_rank"]
		_, hasSparse := item.candidate.Trace["sparse_rank"]
		switch {
		case hasDense && hasSparse:
			item.candidate.Reason = "hybrid_rrf"
		case hasDense:
			item.candidate.Reason = "dense_only_rrf"
		default:
			item.candidate.Reason = "sparse_only_rrf"
		}
		item.candidate.Trace["rrf_score"] = item.candidate.Score
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].candidate.Score != result[j].candidate.Score {
			return result[i].candidate.Score > result[j].candidate.Score
		}
		if result[i].bestRank != result[j].bestRank {
			return result[i].bestRank < result[j].bestRank
		}
		return result[i].candidate.ChunkID < result[j].candidate.ChunkID
	})
	candidates := make([]Candidate, len(result))
	for i := range result {
		candidates[i] = result[i].candidate
	}
	return candidates, nil
}

func validateFusionWeights(weights FusionWeights, rankConstant int) error {
	if rankConstant <= 0 {
		return fmt.Errorf("RRF rank constant must be positive")
	}
	for name, value := range map[string]float64{"dense": weights.Dense, "sparse": weights.Sparse} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s RRF weight must be finite and positive", name)
		}
	}
	return nil
}

func validateCandidateLeg(name string, candidates []Candidate) error {
	seen := map[string]bool{}
	var legScope string
	for index, candidate := range candidates {
		id := strings.TrimSpace(candidate.ChunkID)
		scope := strings.TrimSpace(candidate.KnowledgeScope)
		if id == "" || scope == "" {
			return fmt.Errorf("%s candidate %d identity requires scope and chunk ID", name, index)
		}
		if seen[id] {
			return fmt.Errorf("%s candidate leg contains duplicate chunk ID %q", name, id)
		}
		if legScope == "" {
			legScope = scope
		} else if scope != legScope {
			return fmt.Errorf("%s candidate leg mixes scope %q and %q", name, legScope, scope)
		}
		if math.IsNaN(candidate.Score) || math.IsInf(candidate.Score, 0) {
			return fmt.Errorf("%s candidate %q score must be finite", name, id)
		}
		seen[id] = true
	}
	return nil
}

func mergeCandidateMetadata(target *Candidate, source Candidate) error {
	if target.DocumentID != "" && source.DocumentID != "" && target.DocumentID != source.DocumentID {
		return fmt.Errorf("candidate %q document identity conflict", target.ChunkID)
	}
	if target.Content != "" && source.Content != "" && target.Content != source.Content {
		return fmt.Errorf("candidate %q content conflict", target.ChunkID)
	}
	if target.HeadingPath != "" && source.HeadingPath != "" && target.HeadingPath != source.HeadingPath {
		return fmt.Errorf("candidate %q heading identity conflict", target.ChunkID)
	}
	if target.DocumentID == "" {
		target.DocumentID = source.DocumentID
	}
	if target.Content == "" {
		target.Content = source.Content
	}
	if target.HeadingPath == "" {
		target.HeadingPath = source.HeadingPath
	}
	return nil
}
