package behavior

import (
	"sort"
	"time"
)

type Arbiter struct {
	clock       Clock
	cooldown    *CooldownManager
	mutex       *MutexManager
	fallback    *FallbackGraph
	unavailable *ActionUnavailableCache
}

func NewArbiter(clock Clock, fallback *FallbackGraph) *Arbiter {
	if clock == nil {
		clock = NewRealClock()
	}
	if fallback == nil {
		fallback = DefaultFallbackGraph()
	}
	return &Arbiter{
		clock:       clock,
		cooldown:    NewCooldownManager(clock),
		mutex:       NewMutexManager(),
		fallback:    fallback,
		unavailable: NewActionUnavailableCache(),
	}
}

func (a *Arbiter) Arbitrate(ctx *BehaviorContextSnapshot, candidates []CandidateAction, activePet *ActivePetSnapshot, runtimeOnline bool) (*BehaviorDecision, error) {
	if activePet == nil {
		return &BehaviorDecision{
			Status:     DecisionStatusIgnored,
			ReasonCode: ErrCodeNoActiveInstallation,
			CreatedAt:  a.clock.Now(),
		}, nil
	}

	available := make(map[string]bool)
	for key, cap := range activePet.Actions {
		if cap.Available && !a.unavailable.IsDisabled(key) {
			available[key] = true
		}
	}
	if activePet.DefaultAction != "" {
		available[activePet.DefaultAction] = true
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	now := a.clock.Now()
	var rejected []RejectedCandidate
	selectedIdx := -1

	for i := range candidates {
		c := &candidates[i]
		reasons := a.evaluateCandidate(ctx, c, available, runtimeOnline, now)

		if len(reasons) == 0 {
			selectedIdx = i
			break
		}

		rejected = append(rejected, RejectedCandidate{
			Candidate: *c,
			Reasons:   reasons,
		})
	}

	if selectedIdx < 0 {
		return &BehaviorDecision{
			Status:             DecisionStatusNoAction,
			ReasonCode:         ErrCodeNoActionAvailable,
			RejectedCandidates: rejected,
			CreatedAt:          now,
		}, nil
	}

	winner := candidates[selectedIdx]
	actionKey, depth, err := a.resolveActionKey(winner, available)
	if err != nil {
		rejected = append(rejected, RejectedCandidate{
			Candidate: winner,
			Reasons:   []RejectionReason{RejectMissingAction},
		})
		return &BehaviorDecision{
			Status:             DecisionStatusNoAction,
			ReasonCode:         ErrCodeNoActionAvailable,
			RejectedCandidates: rejected,
			CreatedAt:          now,
		}, nil
	}

	canInterrupt := a.canInterruptCurrent(ctx, &winner, now)

	if !canInterrupt && ctx.Foreground.ActionKey != "" {
		rejected = append(rejected, RejectedCandidate{
			Candidate: winner,
			Reasons:   []RejectionReason{RejectUninterruptible},
		})
		return &BehaviorDecision{
			Status:             DecisionStatusIgnored,
			ReasonCode:         "current_uninterruptible",
			RejectedCandidates: rejected,
			CreatedAt:          now,
		}, nil
	}

	if winner.Cooldown > 0 && winner.CooldownKey != "" {
		a.cooldown.ApplyCooldown(ctx, winner.CooldownKey, winner.Cooldown, "")
	}
	if winner.Semantic != "" {
		a.cooldown.ApplyCooldown(ctx, "semantic:"+winner.Semantic, winner.Cooldown, "")
	}

	interruptPolicy := winner.InterruptPolicy
	if interruptPolicy == "" {
		interruptPolicy = "queue"
	}
	decision := &BehaviorDecision{
		DecisionID:         UUIDNew(),
		EventID:            winner.SourceEventID,
		UserID:             ctx.UserID,
		CharacterID:        ctx.CharacterID,
		InstallationID:     activePet.InstallationID,
		ContextRevision:    ctx.Revision,
		RulesetVersion:     int(CurrentRulesetVersion),
		Semantic:           winner.Semantic,
		ActionKey:          actionKey,
		Priority:           winner.Priority,
		InterruptPolicy:    interruptPolicy,
		MinimumPlayMS:      winner.MinPlay.Milliseconds(),
		MaximumPlayMS:      winner.MaxPlay.Milliseconds(),
		ExpiresAt:          winner.ExpiresAt,
		Status:             DecisionStatusSelected,
		ReasonCode:         "selected",
		RejectedCandidates: rejected,
		FallbackDepth:      depth,
		ReturnPolicy:       winner.ReturnPolicy,
		CreatedAt:          now,
	}

	return decision, nil
}

func (a *Arbiter) evaluateCandidate(ctx *BehaviorContextSnapshot, c *CandidateAction, available map[string]bool, runtimeOnline bool, now time.Time) []RejectionReason {
	var reasons []RejectionReason

	if c.ExpiresAt != nil && now.After(*c.ExpiresAt) {
		reasons = append(reasons, RejectExpired)
		return reasons
	}

	if !runtimeOnline && !c.Durable {
		reliability := getEventReliabilityForSemantic(c.Semantic)
		if reliability == ReliabilityEphemeral {
			reasons = append(reasons, RejectRuntimeOfflineEphem)
			return reasons
		}
	}

	hasAvailable := false
	for _, key := range c.PreferredKeys {
		if available[key] {
			hasAvailable = true
			break
		}
	}
	if !hasAvailable {
		_, _, err := a.fallback.Resolve(c.Semantic, available)
		if err != nil {
			reasons = append(reasons, RejectMissingAction)
			return reasons
		}
	}

	if ok, _ := a.cooldown.CheckCooldown(ctx, *c); !ok {
		reasons = append(reasons, RejectCooldownActive)
	}

	if ctx.Foreground.Semantic != "" && ctx.Foreground.Semantic == c.Semantic {
		if ctx.Foreground.ActionKey != "" {
			reasons = append(reasons, RejectDuplicateSemantic)
		}
	}

	if conflict, _ := a.mutex.FindConflict(c.Semantic, &ctx.Foreground); conflict {
		reasons = append(reasons, RejectMutexConflict)
	}

	return reasons
}

func (a *Arbiter) canInterruptCurrent(ctx *BehaviorContextSnapshot, candidate *CandidateAction, now time.Time) bool {
	if ctx.Foreground.ActionKey == "" {
		return true
	}

	if candidate.InterruptPolicy == "force" {
		return true
	}

	if a.mutex.IsSystemSafety(candidate.Semantic) {
		return true
	}

	if ctx.Foreground.MinPlayUntil != nil && now.Before(*ctx.Foreground.MinPlayUntil) {
		return false
	}

	if !ctx.Foreground.Interruptible {
		return false
	}

	return true
}

func (a *Arbiter) resolveActionKey(c CandidateAction, available map[string]bool) (string, int, error) {
	for _, key := range c.PreferredKeys {
		if available[key] {
			return key, 0, nil
		}
	}
	return a.fallback.Resolve(c.Semantic, available)
}

func (a *Arbiter) MarkActionUnavailable(actionKey string) {
	a.unavailable.MarkUnavailable(actionKey)
}

func (a *Arbiter) ClearUnavailable() {
	a.unavailable.Clear()
}

func (a *Arbiter) ShouldRevertToStable(ctx *BehaviorContextSnapshot) bool {
	if ctx.Foreground.ActionKey == "" {
		return false
	}
	if ctx.Transient.InteractionPhase != "" && ctx.Transient.InteractionPhase != "completed" {
		return false
	}
	if len(ctx.ActiveTools) > 0 {
		return false
	}
	if ctx.Voice.State != "" {
		return false
	}
	if ctx.DesktopGesture.CurrentGesture != "" {
		return false
	}
	return true
}

func (a *Arbiter) ResolveStableRecovery(ctx *BehaviorContextSnapshot, activePet *ActivePetSnapshot) (*BehaviorDecision, error) {
	if activePet == nil {
		return nil, ErrNoActiveInstallation
	}

	available := make(map[string]bool)
	for key, cap := range activePet.Actions {
		if cap.Available {
			available[key] = true
		}
	}

	now := a.clock.Now()

	if ctx.Stable.ActivityKey != "" {
		mapping := map[string][]string{
			"sleep": {"sleep", "sleep_on_desktop", "sit"},
			"rest":  {"sit", "sleep_on_desktop"},
			"eat":   {"eat"},
			"drink": {"drink"},
			"read":  {"read"},
			"write": {"write"},
			"phone": {"use_phone"},
			"work":  {"work"},
			"study": {"study", "read", "work"},
		}
		if keys, ok := mapping[ctx.Stable.ActivityKey]; ok {
			for _, key := range keys {
				if available[key] {
					return &BehaviorDecision{
						DecisionID:      UUIDNew(),
						UserID:          ctx.UserID,
						CharacterID:     ctx.CharacterID,
						InstallationID:  activePet.InstallationID,
						ContextRevision: ctx.Revision,
						RulesetVersion:  int(CurrentRulesetVersion),
						Semantic:        "activity_" + ctx.Stable.ActivityKey,
						ActionKey:       key,
						Priority:        500,
						Status:          DecisionStatusSelected,
						ReasonCode:      "stable_recovery",
						CreatedAt:       now,
					}, nil
				}
			}
		}
	}

	if available["idle_normal"] {
		return &BehaviorDecision{
			DecisionID:      UUIDNew(),
			UserID:          ctx.UserID,
			CharacterID:     ctx.CharacterID,
			InstallationID:  activePet.InstallationID,
			ContextRevision: ctx.Revision,
			RulesetVersion:  int(CurrentRulesetVersion),
			Semantic:        "fallback_idle",
			ActionKey:       "idle_normal",
			Priority:        100,
			Status:          DecisionStatusSelected,
			ReasonCode:      "fallback_idle",
			CreatedAt:       now,
		}, nil
	}

	return &BehaviorDecision{
		Status:     DecisionStatusNoAction,
		ReasonCode: ErrCodeNoActionAvailable,
		CreatedAt:  now,
	}, nil
}

func getEventReliabilityForSemantic(semantic string) EventReliability {
	switch semantic {
	case "gesture_click", "gesture_double_click", "gesture_hover", "gesture_drag":
		return ReliabilityEphemeral
	case "dialogue_speaking", "dialogue_listening", "dialogue_thinking":
		return ReliabilityRecoverable
	case "activity_sleep", "activity_work", "activity_study":
		return ReliabilityDurable
	default:
		return ReliabilityRecoverable
	}
}
