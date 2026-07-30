package behavior

import "time"

type CooldownManager struct {
	clock Clock
}

func NewCooldownManager(clock Clock) *CooldownManager {
	if clock == nil {
		clock = NewRealClock()
	}
	return &CooldownManager{clock: clock}
}

func (m *CooldownManager) IsOnCooldown(ctx *BehaviorContextSnapshot, key string) bool {
	if ctx.Cooldowns == nil {
		return false
	}
	until, ok := ctx.Cooldowns[key]
	if !ok {
		return false
	}
	return m.clock.Now().Before(until)
}

func (m *CooldownManager) ApplyCooldown(ctx *BehaviorContextSnapshot, key string, duration time.Duration, decisionID string) {
	if ctx.Cooldowns == nil {
		ctx.Cooldowns = make(map[string]time.Time)
	}
	ctx.Cooldowns[key] = m.clock.Now().Add(duration)
}

func (m *CooldownManager) CheckCooldown(ctx *BehaviorContextSnapshot, candidate CandidateAction) (bool, RejectionReason) {
	if candidate.CooldownKey == "" || candidate.Cooldown <= 0 {
		return true, ""
	}
	if m.IsOnCooldown(ctx, candidate.CooldownKey) {
		return false, RejectCooldownActive
	}
	if candidate.Semantic != "" && m.IsOnCooldown(ctx, "semantic:"+candidate.Semantic) {
		return false, RejectCooldownActive
	}
	return true, ""
}

func (m *CooldownManager) CleanupExpired(ctx *BehaviorContextSnapshot) {
	if ctx.Cooldowns == nil {
		return
	}
	now := m.clock.Now()
	for key, until := range ctx.Cooldowns {
		if !now.Before(until) {
			delete(ctx.Cooldowns, key)
		}
	}
}

func (m *CooldownManager) RemainingTime(ctx *BehaviorContextSnapshot, key string) time.Duration {
	if ctx.Cooldowns == nil {
		return 0
	}
	until, ok := ctx.Cooldowns[key]
	if !ok {
		return 0
	}
	now := m.clock.Now()
	if !now.Before(until) {
		return 0
	}
	return until.Sub(now)
}

func ActionCooldownKey(actionKey string) string {
	return "action:" + actionKey
}

func SemanticCooldownKey(semantic string) string {
	return "semantic:" + semantic
}

func SourceCooldownKey(origin EventOrigin, sourceID string) string {
	return "source:" + string(origin) + ":" + sourceID
}
