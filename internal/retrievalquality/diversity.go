package retrievalquality

import (
	"fmt"
	"strings"
)

type DiversityLimits struct {
	PerDocument int `json:"per_document"`
	PerHeading  int `json:"per_heading"`
}

type DiversitySkip struct {
	ChunkID string `json:"chunk_id"`
	Reason  string `json:"reason"`
}

func ApplyDiversity(candidates []Candidate, limits DiversityLimits) ([]Candidate, []DiversitySkip, error) {
	if limits.PerDocument <= 0 || limits.PerHeading <= 0 {
		return nil, nil, fmt.Errorf("diversity document and heading limits must be positive")
	}
	if err := validateCandidateLeg("diversity input", candidates); err != nil {
		return nil, nil, err
	}
	documentCounts := map[string]int{}
	headingCounts := map[string]int{}
	kept := make([]Candidate, 0, len(candidates))
	skipped := []DiversitySkip{}
	for _, candidate := range cloneCandidates(candidates) {
		documentID := strings.TrimSpace(candidate.DocumentID)
		headingPath := strings.TrimSpace(candidate.HeadingPath)
		if documentID == "" || headingPath == "" {
			return nil, nil, fmt.Errorf("diversity candidate %q requires document ID and heading path", candidate.ChunkID)
		}
		headingKey := documentID + "\x00" + headingPath
		switch {
		case documentCounts[documentID] >= limits.PerDocument:
			skipped = append(skipped, DiversitySkip{ChunkID: candidate.ChunkID, Reason: "document_limit"})
		case headingCounts[headingKey] >= limits.PerHeading:
			skipped = append(skipped, DiversitySkip{ChunkID: candidate.ChunkID, Reason: "heading_limit"})
		default:
			documentCounts[documentID]++
			headingCounts[headingKey]++
			kept = append(kept, candidate)
		}
	}
	return kept, skipped, nil
}
