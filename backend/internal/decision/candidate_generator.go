package decision

import "time"

type CandidateGenerationContext struct {
	UserID             string
	CharacterID        string
	Goals              []Goal
	Intentions         []Intention
	Psyche             PsycheSignalSet
	Relationship       RelationshipSnapshot
	Life               LifeSnapshot
	Beliefs            BeliefSnapshot
	PersonalityWeights map[BehaviorTag]float64
	Trigger            GoalTrigger
	Now                time.Time
}

func GenerateCandidates(ctx CandidateGenerationContext, registry *CandidateRegistry) []BehaviorCandidate {
	return GenerateCandidatesWithExcludes(ctx, registry, nil)
}

func GenerateCandidatesWithExcludes(ctx CandidateGenerationContext, registry *CandidateRegistry, excludes []string) []BehaviorCandidate {
	if registry == nil {
		return nil
	}
	defs := registry.AllExcept(excludes)
	candidates := make([]BehaviorCandidate, 0, len(defs))
	for _, def := range defs {
		if !candidateAllowedForTrigger(def, ctx.Trigger) {
			continue
		}
		if !candidatePreconditionsSatisfied(def, ctx) {
			continue
		}
		candidates = append(candidates, behaviorFromActionDef(def))
	}
	return candidates
}

func behaviorFromActionDef(def CandidateActionDef) BehaviorCandidate {
	var overrides []string
	if def.Overrides != nil {
		overrides = append([]string(nil), def.Overrides...)
	}
	return BehaviorCandidate{
		ID:         def.ID,
		ActionType: def.Type,
		Tag:        def.Tag,
		Channel:    channelForActionType(def.Type),
		BaseScore:  def.BaseScore,
		Overrides:  overrides,
	}
}

func channelForActionType(actionType CandidateActionType) BehaviorChannel {
	switch actionType {
	case CandidateActionProactive:
		return BehaviorChannelProactive
	case CandidateActionWait:
		return BehaviorChannelSystem
	default:
		return BehaviorChannelChat
	}
}

func candidateAllowedForTrigger(def CandidateActionDef, trigger GoalTrigger) bool {
	if len(def.AllowedTriggers) == 0 {
		return true
	}
	for _, allowed := range def.AllowedTriggers {
		if allowed == trigger.Kind {
			return true
		}
	}
	return false
}

func candidatePreconditionsSatisfied(def CandidateActionDef, ctx CandidateGenerationContext) bool {
	if len(def.Preconds) == 0 {
		return true
	}
	for _, precond := range def.Preconds {
		if !preconditionMet(precond, ctx) {
			return false
		}
	}
	return true
}

func preconditionMet(precond string, ctx CandidateGenerationContext) bool {
	switch precond {
	case "boundary_crossed":
		return hasActiveGoalType(ctx, GoalTypeAutonomy)
	case "information_goal":
		return hasActiveGoalType(ctx, GoalTypeInformation)
	default:
		return false
	}
}

func hasActiveGoalType(ctx CandidateGenerationContext, goalType GoalType) bool {
	for _, goal := range ctx.Goals {
		if goal.Type == goalType && (goal.Status == GoalStatusActive || goal.Status == GoalStatusPending) {
			return true
		}
	}
	for _, intention := range ctx.Intentions {
		if intention.GoalType == goalType && (intention.Status == IntentionStatusExecuting || intention.Status == IntentionStatusFormed) {
			return true
		}
	}
	return false
}
