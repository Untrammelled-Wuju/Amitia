package interaction

import (
	"strings"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

type RuntimeGoalContext struct {
	Current    *decision.Goal       `json:"current,omitempty"`
	Active     []decision.Goal      `json:"active,omitempty"`
	Intentions []decision.Intention `json:"intentions,omitempty"`
}

func (p *RuntimePipeline) buildGoalContext(scope InteractionScope, req *ProcessRequest, appraisal *AppraisalResult, now time.Time) RuntimeGoalContext {
	var registry *decision.GoalRegistry
	if p != nil {
		registry = p.goalRegistry
	}

	var activeGoals []decision.Goal
	if registry != nil {
		activeGoals = registry.ActiveForScope(scope.UserID, scope.CharacterID, scope.ConversationID)
	}

	currentGoal := buildCurrentInteractionGoal(scope, req, appraisal, now)
	if currentGoal == nil {
		return RuntimeGoalContext{
			Active:     activeGoals,
			Intentions: nil,
		}
	}

	allGoals := make([]decision.Goal, 0, len(activeGoals)+1)
	allGoals = append(allGoals, *currentGoal)
	for _, g := range activeGoals {
		if g.ID == currentGoal.ID {
			continue
		}
		allGoals = append(allGoals, g)
	}

	intentions := make([]decision.Intention, 0, len(allGoals))
	for _, g := range allGoals {
		if g.Status != decision.GoalStatusPending && g.Status != decision.GoalStatusActive && g.Status != decision.GoalStatusWish {
			continue
		}
		commitment := decision.CommitmentModerate
		if g.ID == currentGoal.ID {
			commitment = deriveCommitmentFromGoalPriority(g.Priority)
		} else if g.Status == decision.GoalStatusWish {
			commitment = decision.CommitmentWeak
		} else {
			commitment = deriveCommitmentFromGoalPriority(g.Priority)
		}
		intentions = append(intentions, decision.DeriveIntentionAt(g, commitment, time.Time{}, now))
	}

	return RuntimeGoalContext{
		Current:    currentGoal,
		Active:     allGoals,
		Intentions: intentions,
	}
}

func buildCurrentInteractionGoal(scope InteractionScope, req *ProcessRequest, appraisal *AppraisalResult, now time.Time) *decision.Goal {
	if req.InteractionID == "" {
		return nil
	}
	goalID := "goal:interaction:" + req.InteractionID
	trigger := goalTriggerForRequest(scope, req)
	goalType := goalTypeForInteraction(appraisal)
	priority := goalPriorityForInteraction(appraisal)
	description := goalDescriptionForType(goalType)
	return &decision.Goal{
		ID:             goalID,
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Type:           goalType,
		Priority:       priority,
		Status:         decision.GoalStatusActive,
		Progress:       0,
		Description:    description,
		Revision:       1,
		Trigger:        trigger,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func goalTriggerForRequest(scope InteractionScope, req *ProcessRequest) decision.GoalTrigger {
	kind := decision.GoalTriggerUserMessage
	switch {
	case req.IsInternal:
		kind = decision.GoalTriggerInternal
	case strings.TrimSpace(req.ProactiveTaskInstruction) != "":
		kind = decision.GoalTriggerProactive
	case req.VoiceMessage || strings.EqualFold(scope.Source, "voice"):
		kind = decision.GoalTriggerVoice
	}
	return decision.GoalTrigger{
		Kind:          kind,
		Source:        scope.Source,
		RequestID:     scope.RequestID,
		InteractionID: req.InteractionID,
		SessionID:     scope.SessionID,
	}
}

func goalTypeForInteraction(appraisal *AppraisalResult) decision.GoalType {
	if appraisal == nil {
		return decision.GoalTypeConnection
	}
	switch AppraisalEventCategory(appraisal.EventType) {
	case AppraisalCatHelp:
		return decision.GoalTypeSupport
	case AppraisalCatBoundaryCross:
		return decision.GoalTypeAutonomy
	case AppraisalCatApology:
		return decision.GoalTypeConflictRepair
	case AppraisalCatComplaint:
		return decision.GoalTypeConflictRepair
	case AppraisalCatCold:
		return decision.GoalTypeConnection
	case AppraisalCatEmotional:
		return decision.GoalTypeSupport
	case AppraisalCatPraise:
		return decision.GoalTypeConnection
	default:
		return decision.GoalTypeConnection
	}
}

func goalPriorityForInteraction(appraisal *AppraisalResult) decision.GoalPriority {
	if appraisal == nil {
		return decision.GoalPriorityNormal
	}
	if appraisal.Severity >= 0.85 {
		return decision.GoalPriorityCritical
	}
	switch AppraisalEventCategory(appraisal.EventType) {
	case AppraisalCatBoundaryCross:
		return decision.GoalPriorityHigh
	case AppraisalCatComplaint:
		return decision.GoalPriorityHigh
	case AppraisalCatApology:
		return decision.GoalPriorityHigh
	default:
		return decision.GoalPriorityNormal
	}
}

func goalDescriptionForType(goalType decision.GoalType) string {
	switch goalType {
	case decision.GoalTypeSupport:
		return "support_current_interaction"
	case decision.GoalTypeAutonomy:
		return "protect_boundary"
	case decision.GoalTypeConflictRepair:
		return "repair_current_interaction"
	case decision.GoalTypeClarification:
		return "clarify_current_interaction"
	case decision.GoalTypeInformation:
		return "resolve_information_need"
	default:
		return "respond_to_interaction"
	}
}

func deriveCommitmentFromGoalPriority(priority decision.GoalPriority) decision.CommitmentStrength {
	switch priority {
	case decision.GoalPriorityCritical:
		return decision.CommitmentAbsolute
	case decision.GoalPriorityHigh:
		return decision.CommitmentStrong
	case decision.GoalPriorityNormal:
		return decision.CommitmentModerate
	case decision.GoalPriorityLow:
		return decision.CommitmentWeak
	default:
		return decision.CommitmentModerate
	}
}

func goalsForDecision(goalCtx RuntimeGoalContext) []decision.Goal {
	if len(goalCtx.Active) > 0 {
		return goalCtx.Active
	}
	if goalCtx.Current != nil {
		return []decision.Goal{*goalCtx.Current}
	}
	return nil
}
