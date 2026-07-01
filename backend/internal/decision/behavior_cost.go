package decision

import "time"

type BehaviorExecution struct {
	CandidateID string
	ExecutedAt  time.Time
	Cost        float64
}

type BehaviorHistory struct {
	Executions []BehaviorExecution
	MaxHistory int
}

func NewBehaviorHistory(maxHistory int) BehaviorHistory {
	if maxHistory <= 0 {
		maxHistory = 20
	}
	return BehaviorHistory{
		Executions: make([]BehaviorExecution, 0, maxHistory),
		MaxHistory: maxHistory,
	}
}

func (h *BehaviorHistory) Record(candidateID string, executedAt time.Time, cost float64) {
	if h.MaxHistory > 0 && len(h.Executions) >= h.MaxHistory {
		h.Executions = h.Executions[1:]
	}
	h.Executions = append(h.Executions, BehaviorExecution{
		CandidateID: candidateID,
		ExecutedAt:  executedAt,
		Cost:        cost,
	})
}

func (h *BehaviorHistory) CountRecent(candidateID string, since time.Time) int {
	count := 0
	for _, e := range h.Executions {
		if e.CandidateID == candidateID && e.ExecutedAt.After(since) {
			count++
		}
	}
	return count
}

func (h *BehaviorHistory) TotalCost(candidateID string, since time.Time) float64 {
	total := 0.0
	for _, e := range h.Executions {
		if e.CandidateID == candidateID && e.ExecutedAt.After(since) {
			total += e.Cost
		}
	}
	return total
}

func ComputeCumulativeCost(history BehaviorHistory, candidateID string, now time.Time, window time.Duration) float64 {
	if window <= 0 {
		window = 24 * time.Hour
	}
	since := now.Add(-window)
	return history.TotalCost(candidateID, since)
}

type RepeatPenaltyConfig struct {
	Window      time.Duration
	MaxRepeat   int
	PenaltyUnit float64
}

func DefaultRepeatPenaltyConfig() RepeatPenaltyConfig {
	return RepeatPenaltyConfig{
		Window:      30 * time.Minute,
		MaxRepeat:   3,
		PenaltyUnit: 0.15,
	}
}

func ComputeRepeatPenalty(history BehaviorHistory, candidateID string, now time.Time, config RepeatPenaltyConfig) float64 {
	if config.Window <= 0 {
		config.Window = 30 * time.Minute
	}
	if config.MaxRepeat <= 0 {
		config.MaxRepeat = 3
	}
	if config.PenaltyUnit <= 0 {
		config.PenaltyUnit = 0.15
	}
	since := now.Add(-config.Window)
	recentCount := history.CountRecent(candidateID, since)
	if recentCount <= 0 {
		return 0
	}
	penalty := float64(recentCount) * config.PenaltyUnit
	maxPenalty := float64(config.MaxRepeat) * config.PenaltyUnit
	if penalty > maxPenalty {
		penalty = maxPenalty
	}
	return penalty
}

type FatiguePenaltyConfig struct {
	Window         time.Duration
	Threshold      int
	PenaltyPerUnit float64
}

func DefaultFatiguePenaltyConfig() FatiguePenaltyConfig {
	return FatiguePenaltyConfig{
		Window:         1 * time.Hour,
		Threshold:      2,
		PenaltyPerUnit: 0.10,
	}
}

func ComputeFatiguePenalty(history BehaviorHistory, candidateID string, now time.Time, config FatiguePenaltyConfig) float64 {
	if config.Window <= 0 {
		config.Window = 1 * time.Hour
	}
	if config.Threshold <= 0 {
		config.Threshold = 2
	}
	if config.PenaltyPerUnit <= 0 {
		config.PenaltyPerUnit = 0.10
	}
	since := now.Add(-config.Window)
	totalCount := history.CountRecent(candidateID, since)
	if totalCount < config.Threshold {
		return 0
	}
	excess := totalCount - config.Threshold
	penalty := float64(excess) * config.PenaltyPerUnit
	if penalty > 0.5 {
		penalty = 0.5
	}
	return penalty
}
