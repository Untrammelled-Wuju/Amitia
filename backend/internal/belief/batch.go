package belief

import "time"

type BatchInput struct {
	Beliefs []ResolveInput `json:"beliefs"`
}

type BatchResult struct {
	Results []ResolveResult `json:"results"`
}

func ResolveBatch(input BatchInput) BatchResult {
	results := make([]ResolveResult, 0, len(input.Beliefs))
	for _, bi := range input.Beliefs {
		results = append(results, ResolveBelief(bi))
	}
	return BatchResult{Results: results}
}

type ExpandedResolveInput struct {
	Key        string      `json:"key"`
	Candidates []Candidate `json:"candidates"`
	Policy     ResolverPolicy `json:"policy"`
	Now        time.Time   `json:"now"`
}

type MemoryCandidate struct {
	ID         string        `json:"id"`
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Evidence   EvidenceSpan  `json:"evidence"`
	Confidence float64   `json:"confidence"`
	ObservedAt time.Time `json:"observedAt"`
	Source     SourceKind `json:"source"`
}

func ConvertMemoryCandidates(key string, mems []MemoryCandidate, now time.Time, evidence EvidenceSpan) []Candidate {
	_ = evidence
	out := make([]Candidate, 0, len(mems))
	for _, m := range mems {
		expiresAt := m.ObservedAt.Add(72 * time.Hour)
		c := Candidate{
			ID:         m.ID,
			Evidence:   m.Evidence,
			Key:        m.Key,
			Value:      m.Value,
			Source:     m.Source,
			Confidence: m.Confidence,
			ObservedAt: m.ObservedAt,
				ExpiresAt:  expiresAt.Add(0),
		}
		if c.Source == "" {
			c.Source = SourceKindMemory
		}
		out = append(out, c)
	}
	return out
}