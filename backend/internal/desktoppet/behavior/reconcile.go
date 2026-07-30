package behavior

import (
	"context"
	"time"
)

type StateSourceQuery interface {
	QueryActiveInteractions(ctx context.Context, userID, characterID string) ([]InteractionSnapshot, error)
	QueryVoiceSession(ctx context.Context, userID, characterID string) (*VoiceBehaviorState, error)
	QueryActiveTools(ctx context.Context, userID, characterID string) (map[string]ToolOperationState, error)
}

type InteractionSnapshot struct {
	InteractionID  string
	Phase          string
	StatusVersion  int64
	ConversationID string
}

type NoopStateSourceQuery struct{}

func (n *NoopStateSourceQuery) QueryActiveInteractions(_ context.Context, _, _ string) ([]InteractionSnapshot, error) {
	return nil, nil
}
func (n *NoopStateSourceQuery) QueryVoiceSession(_ context.Context, _, _ string) (*VoiceBehaviorState, error) {
	return nil, nil
}
func (n *NoopStateSourceQuery) QueryActiveTools(_ context.Context, _, _ string) (map[string]ToolOperationState, error) {
	return nil, nil
}

type Reconciler struct {
	clock         Clock
	reducer       *Reducer
	resolver      *Resolver
	arbiter       *Arbiter
	stateSource   StateSourceQuery
	affectPort    CharacterAffectPort
	activityPort  CharacterActivityPort
	activePetPort ActivePetPort
	runtimePort   RuntimeActionPort
}

func NewReconciler(
	clock Clock,
	reducer *Reducer,
	resolver *Resolver,
	arbiter *Arbiter,
	stateSource StateSourceQuery,
	affectPort CharacterAffectPort,
	activityPort CharacterActivityPort,
	activePetPort ActivePetPort,
	runtimePort RuntimeActionPort,
) *Reconciler {
	if clock == nil {
		clock = NewRealClock()
	}
	if stateSource == nil {
		stateSource = &NoopStateSourceQuery{}
	}
	return &Reconciler{
		clock:        clock,
		reducer:      reducer,
		resolver:     resolver,
		arbiter:      arbiter,
		stateSource:  stateSource,
		affectPort:   affectPort,
		activityPort: activityPort,
		activePetPort: activePetPort,
		runtimePort:  runtimePort,
	}
}

func (r *Reconciler) ReconcileCharacter(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	now := r.clock.Now()

	activePet, err := r.activePetPort.ResolveActivePet(ctx, userID, characterID)
	if err != nil || activePet == nil {
		return &BehaviorDecision{
			UserID:      userID,
			CharacterID: characterID,
			Status:      DecisionStatusIgnored,
			ReasonCode:  ErrCodeNoActiveInstallation,
			CreatedAt:   now,
		}, nil
	}

	reconciledCtx := r.buildReconciledContext(ctx, userID, characterID, currentContext, now)

	if activePet.RuntimeOnline {
		decision, err := r.arbiter.ResolveStableRecovery(&reconciledCtx, activePet)
		if err != nil {
			return nil, err
		}
		if decision != nil && decision.Status == DecisionStatusSelected {
			r.buildAndSubmitCommand(ctx, decision, activePet, &reconciledCtx)
		}
		return decision, nil
	}

	reconciledCtx.Desired = DesiredBehaviorState{
		Semantic:    "fallback_idle",
		SourceLayer:  "stable",
	}
	if reconciledCtx.Stable.ActivityKey != "" {
		reconciledCtx.Desired = DesiredBehaviorState{
			Semantic:        "activity_" + reconciledCtx.Stable.ActivityKey,
			PreferredAction: "",
			SourceLayer:     "stable",
		}
	}

	return &BehaviorDecision{
		UserID:      userID,
		CharacterID: characterID,
		Status:      DecisionStatusIgnored,
		ReasonCode:  ErrCodeRuntimeOffline,
		CreatedAt:   now,
	}, nil
}

func (r *Reconciler) buildReconciledContext(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot, now time.Time) BehaviorContextSnapshot {
	if currentContext == nil {
		emptyCtx := NewDefaultContext(userID, characterID)
		currentContext = &emptyCtx
	}

	reconciled := currentContext.Copy()

	if interactions, err := r.stateSource.QueryActiveInteractions(ctx, userID, characterID); err == nil && len(interactions) > 0 {
		latest := interactions[0]
		for _, i := range interactions {
			if i.StatusVersion > latest.StatusVersion {
				latest = i
			}
		}
		reconciled.Transient = TransientBehaviorState{
			InteractionID:    latest.InteractionID,
			InteractionPhase: latest.Phase,
			StatusVersion:    latest.StatusVersion,
		}
	} else {
		if reconciled.Transient.InteractionPhase != "completed" && reconciled.Transient.InteractionPhase != "failed" && reconciled.Transient.InteractionPhase != "cancelled" {
			if reconciled.Transient.InteractionPhase != "" {
				reconciled.Transient = TransientBehaviorState{}
			}
		}
	}

	if voiceState, err := r.stateSource.QueryVoiceSession(ctx, userID, characterID); err == nil && voiceState != nil {
		reconciled.Voice = *voiceState
	} else {
		if !reconciled.Voice.LeaseExpiresAt.After(time.Time{}) || now.After(reconciled.Voice.LeaseExpiresAt) {
			reconciled.Voice = VoiceBehaviorState{}
		}
	}

	if tools, err := r.stateSource.QueryActiveTools(ctx, userID, characterID); err == nil && len(tools) > 0 {
		reconciled.ActiveTools = tools
	} else {
		expired := []string{}
		for opID, tool := range reconciled.ActiveTools {
			if now.After(tool.LeaseExpiresAt) {
				expired = append(expired, opID)
			}
		}
		for _, opID := range expired {
			delete(reconciled.ActiveTools, opID)
		}
	}

	if r.affectPort != nil {
		if affect, err := r.affectPort.GetAffectSnapshot(ctx, userID, characterID); err == nil && affect != nil {
			reconciled.Stable.AffectLabel = affect.Label
			reconciled.Stable.AffectVersion = affect.Version
		}
	}

	if r.activityPort != nil {
		if activity, err := r.activityPort.GetActivitySnapshot(ctx, userID, characterID); err == nil && activity != nil {
			reconciled.Stable.ActivityKey = activity.ActivityKey
			reconciled.Stable.ActivitySource = activity.Source
			reconciled.Stable.ActivityConfidence = activity.Confidence
		}
	}

	reconciled.DesktopGesture = DesktopGestureState{}
	reconciled.UpdatedAt = now

	return reconciled
}

func (r *Reconciler) buildAndSubmitCommand(ctx context.Context, decision *BehaviorDecision, activePet *ActivePetSnapshot, ctxSnapshot *BehaviorContextSnapshot) {
	if r.runtimePort == nil {
		return
	}
	cmd := BehaviorRuntimeCommand{
		CommandID:            UUIDNew(),
		DecisionID:           decision.DecisionID,
		IdempotencyKey:       decision.DecisionID,
		PetInstanceID:        activePet.PetInstanceID,
		InstallationID:       activePet.InstallationID,
		InstallationRevision: activePet.StateRevision,
		ContextRevision:      ctxSnapshot.Revision,
		ActionKey:            decision.ActionKey,
		Priority:             decision.Priority,
		ReasonCode:           "reconcile",
	}
	receipt, err := r.runtimePort.SubmitBehaviorCommand(ctx, cmd)
	if err != nil {
		return
	}
	if receipt != nil && receipt.Accepted {
		decision.Status = DecisionStatusCommandSubmitted
		decision.RuntimeCommandID = cmd.CommandID
	}
}

func (r *Reconciler) HandleRuntimeReconnect(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	return r.ReconcileCharacter(ctx, userID, characterID, currentContext)
}

func (r *Reconciler) HandleInstallationChanged(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	r.arbiter.ClearUnavailable()
	currentContext.Foreground = ForegroundActionState{}
	return r.ReconcileCharacter(ctx, userID, characterID, currentContext)
}
