package decision

import "time"

type HardConstraintFilterConfig struct {
	CooldownBlocks map[string]time.Duration
	BlockedTags    []BehaviorTag
	BlockedIDs     []string
}

func DefaultHardConstraintFilterConfig() HardConstraintFilterConfig {
	return HardConstraintFilterConfig{
		CooldownBlocks: map[string]time.Duration{
			"proactive_greet": 5 * time.Minute,
			"express_emotion": 1 * time.Minute,
		},
	}
}

type HardConstraintFilter struct {
	Config HardConstraintFilterConfig
}

func NewHardConstraintFilter(config HardConstraintFilterConfig) HardConstraintFilter {
	return HardConstraintFilter{Config: config}
}

func DefaultHardConstraintFilter() HardConstraintFilter {
	return NewHardConstraintFilter(DefaultHardConstraintFilterConfig())
}

type ConstraintCheckResult struct {
	Allowed bool
	Code    string
	Reason  string
}

func (f *HardConstraintFilter) Check(candidate BehaviorCandidate, now time.Time, history BehaviorHistory) ConstraintCheckResult {
	if isBlockedByID(candidate.ID, f.Config.BlockedIDs) {
		return ConstraintCheckResult{Allowed: false, Code: "blocked_id", Reason: "blocked_id:" + candidate.ID}
	}
	if isBlockedByTag(candidate.Tag, f.Config.BlockedTags) {
		return ConstraintCheckResult{Allowed: false, Code: "blocked_tag", Reason: "blocked_tag:" + string(candidate.Tag)}
	}
	if cooldown, ok := f.Config.CooldownBlocks[candidate.ID]; ok {
		lastAt, found := latestExecutionAt(history, candidate.ID)
		if found && now.Sub(lastAt) < cooldown {
			return ConstraintCheckResult{Allowed: false, Code: "cooldown", Reason: "cooldown:" + candidate.ID}
		}
	}
	if blockedByHardConstraint(candidate) {
		return ConstraintCheckResult{Allowed: false, Code: "hard_constraint_failed", Reason: "hard_constraint_failed"}
	}
	return ConstraintCheckResult{Allowed: true}
}

func (f *HardConstraintFilter) Filter(candidates []BehaviorCandidate, now time.Time, history BehaviorHistory) ([]BehaviorCandidate, []BehaviorCandidate) {
	allowed := make([]BehaviorCandidate, 0, len(candidates))
	blocked := make([]BehaviorCandidate, 0)
	for _, candidate := range candidates {
		check := f.Check(candidate, now, history)
		if check.Allowed {
			allowed = append(allowed, candidate)
		} else {
			next := cloneCandidate(candidate)
			next.Reasons = append(next.Reasons, BehaviorReason{Source: "arbitration", Key: "hard_constraint:" + check.Code, Delta: 0})
			blocked = append(blocked, next)
		}
	}
	return allowed, blocked
}

func latestExecutionAt(history BehaviorHistory, candidateID string) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, e := range history.Executions {
		if e.CandidateID == candidateID {
			if !found || e.ExecutedAt.After(latest) {
				latest = e.ExecutedAt
				found = true
			}
		}
	}
	return latest, found
}

func isBlockedByID(id string, blockedIDs []string) bool {
	for _, blocked := range blockedIDs {
		if blocked == id {
			return true
		}
	}
	return false
}

func isBlockedByTag(tag BehaviorTag, blockedTags []BehaviorTag) bool {
	for _, blocked := range blockedTags {
		if blocked == tag {
			return true
		}
	}
	return false
}
