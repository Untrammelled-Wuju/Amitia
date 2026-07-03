package budget

import "math"

type BudgetController struct {
	MaxTotalDelta float64
}

func NewBudgetController(maxTotal float64) *BudgetController {
	return &BudgetController{MaxTotalDelta: maxTotal}
}

type CandidateDelta struct {
	Module   string
	Delta    float64
	Priority int
	Reason   string
}

type BudgetResult struct {
	OriginalCandidates []CandidateDelta
	FinalDeltas        []CandidateDelta
	Rejected           []CandidateDelta
	TotalAllocated     float64
}

func (bc *BudgetController) Allocate(severity float64, candidates []CandidateDelta) BudgetResult {
	totalBudget := severity * bc.MaxTotalDelta
	if totalBudget <= 0 {
		totalBudget = 0.01
	}

	result := BudgetResult{
		OriginalCandidates: candidates,
		FinalDeltas:        []CandidateDelta{},
		Rejected:           []CandidateDelta{},
	}

	remaining := totalBudget
	type candidateEntry struct {
		c CandidateDelta
		d float64
	}
	var entries []candidateEntry
	for _, c := range candidates {
		entries = append(entries, candidateEntry{c, math.Abs(c.Delta)})
	}

	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].c.Priority < entries[i].c.Priority {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for _, s := range entries {
		c := s.c
		absDelta := math.Abs(c.Delta)
		if remaining >= absDelta {
			result.FinalDeltas = append(result.FinalDeltas, c)
			result.TotalAllocated += absDelta
			remaining -= absDelta
		} else if remaining > 0 {
			scaled := c
			sign := 1.0
			if c.Delta < 0 {
				sign = -1.0
			}
			scaled.Delta = sign * remaining
			result.FinalDeltas = append(result.FinalDeltas, scaled)
			result.TotalAllocated += remaining
			remaining = 0
		} else {
			result.Rejected = append(result.Rejected, c)
		}
	}

	return result
}

func ComputeEventSeverity(appraisalSeverity float64, goalRelevance float64, normViolation float64, boundaryViolation float64) float64 {
	severity := 0.3*appraisalSeverity + 0.25*goalRelevance + 0.2*normViolation + 0.25*boundaryViolation
	return clamp01(severity)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
