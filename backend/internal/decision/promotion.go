package decision

import "math"

const promotionStep = 0.05

func PromoteCandidates(candidates []BehaviorCandidate, ids []string) []BehaviorCandidate {
	if len(candidates) == 0 || len(ids) == 0 {
		return candidates
	}
	targets := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id != "" {
			targets[id] = true
		}
	}
	if len(targets) == 0 {
		return candidates
	}
	highest := math.Inf(-1)
	for _, candidate := range candidates {
		if candidate.FinalScore > highest {
			highest = candidate.FinalScore
		}
	}
	result := make([]BehaviorCandidate, len(candidates))
	copy(result, candidates)
	promoted := 0
	for i := range result {
		if !targets[result[i].ID] {
			continue
		}
		promoted++
		score := round4(highest + promotionStep*float64(promoted))
		delta := round4(score - result[i].FinalScore)
		result[i].FinalScore = score
		result[i].Reasons = append(result[i].Reasons, BehaviorReason{Source: "safety_bdi", Key: "override", Delta: delta})
	}
	return result
}
