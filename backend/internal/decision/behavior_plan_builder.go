package decision

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

func NewBehaviorPlanID() string {
	return "plan:" + uuid.NewString()
}

type BehaviorPlanBuildInput struct {
	PlanID          string
	UserID          string
	CharacterID     string
	ConversationID  string
	InteractionID   string
	RequestID       string
	Arbitration     ArbitrationResult
	Goals           []Goal
	Intentions      []Intention
	Psyche          PsycheSignalSet
	Relationship    RelationshipSnapshot
	Life            LifeSnapshot
	Personality     CompiledPersonalityRef
	Safety          PlanSafetyContext
	ContentPolicy   PlanContentPolicy
	Now             time.Time
}

type BehaviorPlanBuilder struct{}

func NewBehaviorPlanBuilder() BehaviorPlanBuilder {
	return BehaviorPlanBuilder{}
}

func (b BehaviorPlanBuilder) Build(input BehaviorPlanBuildInput) (*BehaviorPlan, error) {
	if !input.Arbitration.HasSelection {
		return nil, nil
	}
	if input.Now.IsZero() {
		return nil, errors.New("plan: Now is required")
	}
	if err := ValidateArbitrationConfig(DefaultArbitrationConfig()); err == nil {
		candidate := input.Arbitration.Selected
		if candidate.ScoringVersion != "" && input.Arbitration.Audit.FormulaVersion != "" &&
			candidate.ScoringVersion != input.Arbitration.Audit.FormulaVersion {
			return nil, fmt.Errorf("plan: candidate ScoringVersion=%s does not match Audit.FormulaVersion=%s",
				candidate.ScoringVersion, input.Arbitration.Audit.FormulaVersion)
		}
	}

	candidate := input.Arbitration.Selected
	planID := input.PlanID
	if planID == "" {
		planID = NewBehaviorPlanID()
	}

	priority := derivePlanPriority(candidate, input.Goals)
	safetyLevel, doNotSend, needsExpression := derivePlanOutputPolicy(candidate, input.Safety)

	intent := derivePlanIntent(candidate)
	strategy := derivePlanStrategy(candidate)
	toneHint := derivePlanToneHint(candidate, input.Psyche, input.Relationship)
	responseGoal := derivePlanResponseGoal(candidate)
	goalIDs, intentionIDs := collectMatchingGoalIntentionIDs(candidate, input.Goals, input.Intentions)

	expressionPlanID := ""
	if needsExpression && !doNotSend {
		expressionPlanID = "expr:" + planID
	}

	audit := cloneBehaviorAudit(input.Arbitration.Audit)
	audit.Diagnostics = append(audit.Diagnostics, "plan:selected:"+candidate.ID)
	if input.Arbitration.Disposition == ArbitrationDispositionFallback {
		audit.Diagnostics = append(audit.Diagnostics, "plan:fallback")
	}
	if doNotSend {
		audit.Diagnostics = append(audit.Diagnostics, "plan:do_not_send")
	}
	if input.Safety.Blocked {
		audit.Diagnostics = append(audit.Diagnostics, "plan:safety_blocked")
	}

	plan := &BehaviorPlan{
		Version:              PlanVersionV2,
		ID:                   planID,
		UserID:               input.UserID,
		CharacterID:          input.CharacterID,
		ConversationID:       input.ConversationID,
		InteractionID:        input.InteractionID,
		RequestID:            input.RequestID,
		CreatedAt:            input.Now,
		Selected:             cloneBehaviorCandidate(candidate),
		Alternatives:         cloneBehaviorCandidates(input.Arbitration.Alternatives),
		SelectionDisposition: input.Arbitration.Disposition,
		Priority:             priority,
		SafetyLevel:          safetyLevel,
		DoNotSend:            doNotSend,
		NeedsExpression:      needsExpression,
		ExpressionPlanID:     expressionPlanID,
		Personality:          cloneCompiledPersonalityRef(input.Personality),
		Psyche:               clonePsycheSignalSet(input.Psyche),
		Relationship:         cloneRelationshipSnapshot(input.Relationship),
		Life:                 cloneLifeSnapshot(input.Life),
		Audit:                audit,
		Intent:               intent,
		Strategy:             strategy,
		PlanContentPolicy:    clonePlanContentPolicy(input.ContentPolicy),
		ResponseGoal:         responseGoal,
		ToneHint:             toneHint,
		GoalIDs:              goalIDs,
		IntentionIDs:         intentionIDs,
	}

	if err := validatePlanConsistency(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func derivePlanPriority(candidate BehaviorCandidate, goals []Goal) BehaviorPriority {
	highestGoalPriority := GoalPriorityNormal
	found := false
	for _, goal := range goals {
		if goal.Status != GoalStatusActive && goal.Status != GoalStatusPending {
			continue
		}
		boost := mapGoalToBoost(goal.Type, candidate.ID)
		if boost > 0 {
			if !found || goalPriorityRank(goal.Priority) > goalPriorityRank(highestGoalPriority) {
				highestGoalPriority = goal.Priority
				found = true
			}
		}
	}

	if found {
		return behaviorPriorityFromGoal(highestGoalPriority)
	}

	switch candidate.Tag {
	case BehaviorTagSetBoundary, BehaviorTagRepair:
		return BehaviorPriorityHigh
	case BehaviorTagAskClarify, BehaviorTagReply, BehaviorTagOfferSupport:
		return BehaviorPriorityNormal
	case BehaviorTagProactiveCheck:
		return BehaviorPriorityNormal
	case BehaviorTagDelay:
		return BehaviorPriorityLow
	default:
		return BehaviorPriorityNormal
	}
}

func behaviorPriorityFromGoal(gp GoalPriority) BehaviorPriority {
	switch gp {
	case GoalPriorityCritical:
		return BehaviorPriorityCritical
	case GoalPriorityHigh:
		return BehaviorPriorityHigh
	case GoalPriorityLow:
		return BehaviorPriorityLow
	default:
		return BehaviorPriorityNormal
	}
}

func goalPriorityRank(gp GoalPriority) int {
	switch gp {
	case GoalPriorityCritical:
		return 4
	case GoalPriorityHigh:
		return 3
	case GoalPriorityNormal:
		return 2
	case GoalPriorityLow:
		return 1
	default:
		return 2
	}
}

func derivePlanOutputPolicy(candidate BehaviorCandidate, safety PlanSafetyContext) (BehaviorSafetyLevel, bool, bool) {
	if safety.Blocked {
		return BehaviorSafetyLevelBlocked, true, false
	}

	safetyLevel := BehaviorSafetyLevelNormal
	switch safety.Level {
	case BehaviorSafetyLevelBlocked:
		return BehaviorSafetyLevelBlocked, true, false
	case BehaviorSafetyLevelConservative:
		safetyLevel = BehaviorSafetyLevelConservative
	}

	if candidate.RiskScore >= 0.5 && safetyLevel == BehaviorSafetyLevelNormal {
		safetyLevel = BehaviorSafetyLevelConservative
	}

	switch candidate.ActionType {
	case CandidateActionToolCall:
		return safetyLevel, true, false
	}

	switch candidate.Tag {
	case BehaviorTagDelay:
		return safetyLevel, true, false
	default:
		return safetyLevel, false, true
	}
}

func derivePlanIntent(candidate BehaviorCandidate) BehaviorPlanIntent {
	switch candidate.Tag {
	case BehaviorTagAskClarify:
		return PlanIntentClarify
	case BehaviorTagOfferSupport:
		return PlanIntentSupport
	case BehaviorTagSetBoundary:
		return PlanIntentBoundary
	case BehaviorTagRepair:
		return PlanIntentRepair
	case BehaviorTagProactiveCheck:
		return PlanIntentProactive
	case BehaviorTagDelay:
		return PlanIntentObserve
	case BehaviorTagReply:
		return PlanIntentReply
	case "tool_call":
		return PlanIntentTool
	default:
		switch candidate.ActionType {
		case CandidateActionToolCall:
			return PlanIntentTool
		}
		return PlanIntentReply
	}
}

func derivePlanStrategy(candidate BehaviorCandidate) BehaviorPlanStrategy {
	switch candidate.Tag {
	case BehaviorTagReply:
		return StrategyRespondNaturally
	case BehaviorTagAskClarify:
		return StrategyRequestClarification
	case BehaviorTagOfferSupport:
		return StrategyProvideSupport
	case BehaviorTagSetBoundary:
		return StrategySetBoundary
	case BehaviorTagRepair:
		return StrategyRepairRelationship
	case BehaviorTagProactiveCheck:
		return StrategyProactiveCheck
	case BehaviorTagDelay:
		return StrategyObserveWithoutResponse
	default:
		switch candidate.ActionType {
		case CandidateActionToolCall:
			return StrategyResolveViaTool
		}
		return StrategyRespondNaturally
	}
}

func derivePlanResponseGoal(candidate BehaviorCandidate) string {
	switch candidate.Tag {
	case BehaviorTagReply:
		return "continue_conversation"
	case BehaviorTagAskClarify:
		return "clarify_understanding"
	case BehaviorTagOfferSupport:
		return "support_user"
	case BehaviorTagSetBoundary:
		return "protect_boundary"
	case BehaviorTagRepair:
		return "repair_relationship"
	case BehaviorTagProactiveCheck:
		return "check_in"
	case BehaviorTagDelay:
		return "observe"
	default:
		switch candidate.ActionType {
		case CandidateActionToolCall:
			return "resolve_information"
		}
		return "continue_conversation"
	}
}

func derivePlanToneHint(candidate BehaviorCandidate, psyche PsycheSignalSet, rel RelationshipSnapshot) ExpressionTone {
	if candidate.Tag == BehaviorTagSetBoundary {
		return ExpressionToneFirm
	}
	stressVal := psyche.Stress.Value
	if stressVal > 0.7 {
		return ExpressionToneSoft
	}
	if psyche.Mood.Value < 0.3 {
		return ExpressionToneWarm
	}
	if candidate.Tag == BehaviorTagOfferSupport || candidate.Tag == BehaviorTagProactiveCheck {
		return ExpressionToneWarm
	}
	if candidate.Tag == BehaviorTagRepair {
		return ExpressionToneSoft
	}
	if candidate.Tag == BehaviorTagAskClarify {
		return ExpressionToneNeutral
	}
	if candidate.Tag == BehaviorTagDelay {
		return ExpressionToneNeutral
	}
	if rel.Dimensions != nil {
		if v, ok := rel.Dimensions[RelationshipFamiliarity]; ok && v.Value > 0.6 {
			return ExpressionToneWarm
		}
	}
	return ExpressionToneNeutral
}

func collectMatchingGoalIntentionIDs(candidate BehaviorCandidate, goals []Goal, intentions []Intention) ([]string, []string) {
	goalIDs := make([]string, 0)
	for _, goal := range goals {
		if goal.Status != GoalStatusActive && goal.Status != GoalStatusPending {
			continue
		}
		if mapGoalToBoost(goal.Type, candidate.ID) > 0 {
			goalIDs = append(goalIDs, goal.ID)
		}
	}
	intentionIDs := make([]string, 0)
	for _, intent := range intentions {
		if intent.Status != IntentionStatusFormed && intent.Status != IntentionStatusExecuting {
			continue
		}
		if mapIntentionToBoost(intent, candidate.ID) > 0 {
			intentionIDs = append(intentionIDs, intent.GoalID)
		}
	}
	return goalIDs, intentionIDs
}

func validatePlanConsistency(plan *BehaviorPlan) error {
	if plan.Version != PlanVersionV2 {
		return fmt.Errorf("plan: version must be V2, got %s", plan.Version)
	}
	if plan.ID == "" {
		return errors.New("plan: ID cannot be empty")
	}
	if plan.Selected.ID == "" {
		return errors.New("plan: Selected.ID cannot be empty")
	}
	if plan.Selected.ScoringVersion == "" {
		return errors.New("plan: Selected.ScoringVersion must be set")
	}
	if plan.DoNotSend && plan.NeedsExpression {
		return errors.New("plan: DoNotSend and NeedsExpression cannot both be true")
	}
	if plan.NeedsExpression && plan.ExpressionPlanID == "" {
		return errors.New("plan: NeedsExpression=true requires ExpressionPlanID")
	}
	if !plan.NeedsExpression && plan.ExpressionPlanID != "" {
		return errors.New("plan: NeedsExpression=false requires empty ExpressionPlanID")
	}
	if plan.SafetyLevel == BehaviorSafetyLevelBlocked && (!plan.DoNotSend || !plan.NeedsExpression) {
	}
	return nil
}

func cloneBehaviorAudit(audit BehaviorAudit) BehaviorAudit {
	clone := BehaviorAudit{
		FormulaVersion:   audit.FormulaVersion,
		ParameterVersion: audit.ParameterVersion,
		SnapshotID:       audit.SnapshotID,
		ReplayEventID:    audit.ReplayEventID,
	}
	if audit.ConflictIDs != nil {
		clone.ConflictIDs = append([]string(nil), audit.ConflictIDs...)
	}
	if audit.Diagnostics != nil {
		clone.Diagnostics = append([]string(nil), audit.Diagnostics...)
	}
	return clone
}

func clonePlanContentPolicy(policy PlanContentPolicy) PlanContentPolicy {
	clone := PlanContentPolicy{}
	if policy.AllowedTopics != nil {
		clone.AllowedTopics = append([]string(nil), policy.AllowedTopics...)
	}
	if policy.ForbiddenTopics != nil {
		clone.ForbiddenTopics = append([]string(nil), policy.ForbiddenTopics...)
	}
	return clone
}
