package decision

import "time"

type HardConstraintFilterConfig struct {
	CooldownBlocks map[string]time.Duration
	SafetyCutoff   float64
	BlockedTags    []BehaviorTag
	BlockedIDs     []string
}

func DefaultHardConstraintFilterConfig() HardConstraintFilterConfig {
	return HardConstraintFilterConfig{
		CooldownBlocks: map[string]time.Duration{
			"proactive_greet": 5 * time.Minute,
			"express_emotion": 1 * time.Minute,
		},
		SafetyCutoff: 0.15,
	}
}

type CooldownRecord struct {
	CandidateID string
	LastSentAt  time.Time
}

type CooldownTracker struct {
	records map[string]CooldownRecord
}

func NewCooldownTracker() CooldownTracker {
	return CooldownTracker{
		records: make(map[string]CooldownRecord),
	}
}

func (t *CooldownTracker) MarkSent(candidateID string, now time.Time) {
	t.records[candidateID] = CooldownRecord{
		CandidateID: candidateID,
		LastSentAt:  now,
	}
}

func (t *CooldownTracker) IsInCooldown(candidateID string, cooldown time.Duration, now time.Time) bool {
	record, ok := t.records[candidateID]
	if !ok {
		return false
	}
	return now.Before(record.LastSentAt.Add(cooldown))
}

type HardConstraintFilter struct {
	Config   HardConstraintFilterConfig
	Cooldown CooldownTracker
}

func NewHardConstraintFilter(config HardConstraintFilterConfig) HardConstraintFilter {
	return HardConstraintFilter{
		Config:   config,
		Cooldown: NewCooldownTracker(),
	}
}

func DefaultHardConstraintFilter() HardConstraintFilter {
	return NewHardConstraintFilter(DefaultHardConstraintFilterConfig())
}

type ConstraintCheckResult struct {
	Allowed bool
	Reason  string
}

func (f *HardConstraintFilter) Check(candidate BehaviorCandidate, now time.Time) ConstraintCheckResult {
	if isBlockedByID(candidate.ID, f.Config.BlockedIDs) {
		return ConstraintCheckResult{Allowed: false, Reason: "blocked_id:" + candidate.ID}
	}
	if isBlockedByTag(candidate.Tag, f.Config.BlockedTags) {
		return ConstraintCheckResult{Allowed: false, Reason: "blocked_tag:" + string(candidate.Tag)}
	}
	if cooldown, ok := f.Config.CooldownBlocks[candidate.ID]; ok {
		if f.Cooldown.IsInCooldown(candidate.ID, cooldown, now) {
			return ConstraintCheckResult{Allowed: false, Reason: "cooldown:" + candidate.ID}
		}
	}
	if blockedByHardConstraint(candidate) {
		return ConstraintCheckResult{Allowed: false, Reason: "hard_constraint_failed"}
	}
	return ConstraintCheckResult{Allowed: true}
}

func (f *HardConstraintFilter) Filter(candidates []BehaviorCandidate, now time.Time) ([]BehaviorCandidate, []BehaviorCandidate) {
	allowed := make([]BehaviorCandidate, 0, len(candidates))
	blocked := make([]BehaviorCandidate, 0)
	for _, candidate := range candidates {
		check := f.Check(candidate, now)
		if check.Allowed {
			allowed = append(allowed, candidate)
		} else {
			next := candidate
			next.Reasons = append(next.Reasons, BehaviorReason{Source: "hard_constraint", Key: check.Reason, Delta: 0})
			blocked = append(blocked, next)
		}
	}
	return allowed, blocked
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
