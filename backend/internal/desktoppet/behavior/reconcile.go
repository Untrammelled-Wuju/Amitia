package behavior

import (
	"context"
	"errors"
	"fmt"
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
	repo          BehaviorStateRepository
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
	repo BehaviorStateRepository,
) *Reconciler {
	if clock == nil {
		clock = NewRealClock()
	}
	if stateSource == nil {
		stateSource = &NoopStateSourceQuery{}
	}
	fallback := DefaultFallbackGraph()
	if reducer == nil {
		reducer = NewReducer(clock)
	}
	if resolver == nil {
		resolver = NewResolver(clock, fallback)
	}
	if arbiter == nil {
		arbiter = NewArbiter(clock, fallback)
	}
	return &Reconciler{
		clock:         clock,
		reducer:       reducer,
		resolver:      resolver,
		arbiter:       arbiter,
		stateSource:   stateSource,
		affectPort:    affectPort,
		activityPort:  activityPort,
		activePetPort: activePetPort,
		runtimePort:   runtimePort,
		repo:          repo,
	}
}

func (r *Reconciler) ReconcileCharacter(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	if userID == "" || characterID == "" {
		return nil, NewBehaviorError(ErrCodeEventSchemaInvalid, "reconcile requires userId and characterId")
	}
	now := r.clock.Now()

	var activePet *ActivePetSnapshot
	var activePetErr error
	if r.activePetPort != nil {
		activePet, activePetErr = r.activePetPort.ResolveActivePet(ctx, userID, characterID)
	} else {
		activePetErr = NewBehaviorError(ErrCodeNoActiveInstallation, "active pet port unavailable")
	}
	if activePetErr != nil && !IsErrorCode(activePetErr, ErrCodeNoActiveInstallation) {
		return nil, activePetErr
	}

	reconciledCtx, err := r.rebuildAndPersistContext(ctx, userID, characterID, currentContext, activePet, now)
	if err != nil {
		return nil, err
	}

	if activePet == nil || activePetErr != nil {
		return &BehaviorDecision{
			UserID:          userID,
			CharacterID:     characterID,
			ContextRevision: reconciledCtx.Revision,
			RulesetVersion:  int(CurrentRulesetVersion),
			Status:          DecisionStatusIgnored,
			ReasonCode:      ErrCodeNoActiveInstallation,
			CreatedAt:       now,
		}, nil
	}

	if !activePet.RuntimeOnline {
		return &BehaviorDecision{
			UserID:          userID,
			CharacterID:     characterID,
			InstallationID:  activePet.InstallationID,
			ContextRevision: reconciledCtx.Revision,
			RulesetVersion:  int(CurrentRulesetVersion),
			Status:          DecisionStatusIgnored,
			ReasonCode:      ErrCodeRuntimeOffline,
			CreatedAt:       now,
		}, nil
	}

	decision, err := r.arbiter.ResolveStableRecovery(&reconciledCtx, activePet)
	if err != nil {
		return nil, err
	}
	if decision == nil {
		return nil, NewBehaviorError(ErrCodeRulesetInvalid, "stable recovery returned nil decision")
	}
	if decision.DecisionID == "" {
		decision.DecisionID = UUIDNew()
	}
	decision.UserID = userID
	decision.CharacterID = characterID
	decision.InstallationID = activePet.InstallationID
	decision.ContextRevision = reconciledCtx.Revision
	if decision.RulesetVersion == 0 {
		decision.RulesetVersion = int(CurrentRulesetVersion)
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = now
	}

	if r.repo != nil {
		if err := r.repo.AppendDecision(ctx, BehaviorDecisionAudit{
			BehaviorDecision: *decision,
			ContextHash:      hashContext(&reconciledCtx),
		}); err != nil {
			return nil, fmt.Errorf("persist reconcile decision: %w", err)
		}
	}

	if decision.Status == DecisionStatusSelected {
		if err := r.buildAndSubmitCommand(ctx, decision, activePet, &reconciledCtx); err != nil {
			if r.repo != nil {
				now := r.clock.Now()
				decision.Status = DecisionStatusFailed
				decision.CompletedAt = &now
				if decision.ReasonCode == "" || decision.ReasonCode == "stable_recovery" || decision.ReasonCode == "fallback_idle" {
					decision.ReasonCode = behaviorProcessingErrorCode(err)
				}
				if persistErr := r.repo.UpdateDecisionOutcome(ctx, *decision); persistErr != nil {
					return nil, errors.Join(err, fmt.Errorf("persist reconcile failure outcome: %w", persistErr))
				}
			}
			return nil, err
		}
	}
	return decision, nil
}

func (r *Reconciler) rebuildAndPersistContext(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot, activePet *ActivePetSnapshot, now time.Time) (BehaviorContextSnapshot, error) {
	base := currentContext
	maxRetries := MaxCASRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return BehaviorContextSnapshot{}, err
		}
		if base == nil || attempt > 0 {
			if r.repo != nil {
				loaded, err := r.repo.LoadContext(ctx, userID, characterID)
				if err != nil {
					return BehaviorContextSnapshot{}, err
				}
				base = loaded
			} else {
				empty := NewDefaultContext(userID, characterID)
				base = &empty
			}
		}

		reconciled, err := r.buildReconciledContext(ctx, userID, characterID, base, now)
		if err != nil {
			return BehaviorContextSnapshot{}, err
		}
		if activePet == nil || !activePet.RuntimeOnline {
			applyStableDesiredState(&reconciled)
		}
		reconciled.Revision = base.Revision + 1
		reconciled.UpdatedAt = now

		if r.repo == nil {
			return reconciled, nil
		}
		committed, err := r.repo.SaveContextCAS(ctx, base.Revision, reconciled)
		if err != nil {
			return BehaviorContextSnapshot{}, fmt.Errorf("persist reconciled context: %w", err)
		}
		if committed {
			return reconciled, nil
		}
	}
	return BehaviorContextSnapshot{}, NewBehaviorError(ErrCodeContextConflict, "reconcile context CAS retries exhausted")
}

func applyStableDesiredState(ctx *BehaviorContextSnapshot) {
	if ctx == nil {
		return
	}
	ctx.Desired = DesiredBehaviorState{
		Semantic:    "fallback_idle",
		SourceLayer: "stable",
	}
	if ctx.Stable.ActivityKey != "" {
		ctx.Desired = DesiredBehaviorState{
			Semantic:    "activity_" + ctx.Stable.ActivityKey,
			SourceLayer: "stable",
		}
	}
}

func (r *Reconciler) buildReconciledContext(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot, now time.Time) (BehaviorContextSnapshot, error) {
	if currentContext == nil {
		emptyCtx := NewDefaultContext(userID, characterID)
		currentContext = &emptyCtx
	}

	reconciled := currentContext.Copy()

	interactions, err := r.stateSource.QueryActiveInteractions(ctx, userID, characterID)
	if err != nil {
		return BehaviorContextSnapshot{}, err
	}
	if len(interactions) > 0 {
		latest := interactions[0]
		for _, interaction := range interactions {
			if interaction.StatusVersion > latest.StatusVersion {
				latest = interaction
			}
		}
		reconciled.Transient = TransientBehaviorState{
			InteractionID:    latest.InteractionID,
			InteractionPhase: latest.Phase,
			StatusVersion:    latest.StatusVersion,
		}
	} else if reconciled.Transient.InteractionPhase != "completed" && reconciled.Transient.InteractionPhase != "failed" && reconciled.Transient.InteractionPhase != "cancelled" {
		if reconciled.Transient.InteractionPhase != "" {
			reconciled.Transient = TransientBehaviorState{}
		}
	}

	voiceState, err := r.stateSource.QueryVoiceSession(ctx, userID, characterID)
	if err != nil {
		return BehaviorContextSnapshot{}, err
	}
	if voiceState != nil {
		reconciled.Voice = *voiceState
	} else if reconciled.Voice.LeaseExpiresAt.IsZero() || now.After(reconciled.Voice.LeaseExpiresAt) {
		reconciled.Voice = VoiceBehaviorState{}
	}

	tools, err := r.stateSource.QueryActiveTools(ctx, userID, characterID)
	if err != nil {
		return BehaviorContextSnapshot{}, err
	}
	if len(tools) > 0 {
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
		affect, err := r.affectPort.GetAffectSnapshot(ctx, userID, characterID)
		if err != nil {
			return BehaviorContextSnapshot{}, fmt.Errorf("query affect snapshot: %w", err)
		}
		if affect != nil {
			reconciled.Stable.AffectLabel = affect.Label
			reconciled.Stable.AffectVersion = affect.Version
		}
	}

	if r.activityPort != nil {
		activity, err := r.activityPort.GetActivitySnapshot(ctx, userID, characterID)
		if err != nil {
			return BehaviorContextSnapshot{}, fmt.Errorf("query activity snapshot: %w", err)
		}
		if activity != nil {
			reconciled.Stable.ActivityKey = activity.ActivityKey
			reconciled.Stable.ActivitySource = activity.Source
			reconciled.Stable.ActivityConfidence = activity.Confidence
			reconciled.Stable.ActivityVersion = activity.Version
		}
	}

	reconciled.DesktopGesture = DesktopGestureState{}
	reconciled.UpdatedAt = now
	return reconciled, nil
}

func (r *Reconciler) buildAndSubmitCommand(ctx context.Context, decision *BehaviorDecision, activePet *ActivePetSnapshot, ctxSnapshot *BehaviorContextSnapshot) error {
	if r.runtimePort == nil {
		return NewBehaviorError(ErrCodeRuntimeOffline, "runtime action port unavailable")
	}
	if decision.DecisionID == "" {
		decision.DecisionID = UUIDNew()
	}
	cmd := BehaviorRuntimeCommand{
		CommandID:            UUIDNew(),
		DecisionID:           decision.DecisionID,
		IdempotencyKey:       decision.DecisionID,
		UserID:               activePet.UserID,
		DeviceID:             activePet.DeviceID,
		CharacterID:          activePet.CharacterID,
		RuntimeID:            activePet.RuntimeID,
		PetInstanceID:        activePet.PetInstanceID,
		InstallationID:       activePet.InstallationID,
		InstallationRevision: activePet.StateRevision,
		ContextRevision:      ctxSnapshot.Revision,
		ActionKey:            decision.ActionKey,
		Semantic:             decision.Semantic,
		Priority:             decision.Priority,
		InterruptPolicy:      decision.InterruptPolicy,
		MinimumPlayMS:        decision.MinimumPlayMS,
		MaximumPlayMS:        decision.MaximumPlayMS,
		ExpiresAt:            decision.ExpiresAt,
		ReturnPolicy:         decision.ReturnPolicy,
		ReasonCode:           "reconcile",
		Durable:              true,
	}
	receipt, err := r.runtimePort.SubmitBehaviorCommand(ctx, cmd)
	if err != nil {
		return err
	}
	if receipt == nil || !receipt.Accepted {
		reason := ErrCodeRuntimeCommandFailed
		if receipt != nil && receipt.PendingReason != "" {
			reason = receipt.PendingReason
		}
		return NewBehaviorError(reason, "reconcile runtime command rejected")
	}
	decision.Status = DecisionStatusCommandSubmitted
	decision.RuntimeCommandID = receipt.CommandID
	if decision.RuntimeCommandID == "" {
		decision.RuntimeCommandID = cmd.CommandID
	}
	now := r.clock.Now()
	decision.StartedAt = &now
	if r.repo != nil {
		if err := r.repo.UpdateDecisionOutcome(ctx, *decision); err != nil {
			return fmt.Errorf("persist reconcile command outcome: %w", err)
		}
	}
	return nil
}

func (r *Reconciler) HandleRuntimeReconnect(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	return r.ReconcileCharacter(ctx, userID, characterID, currentContext)
}

func (r *Reconciler) HandleInstallationChanged(ctx context.Context, userID, characterID string, currentContext *BehaviorContextSnapshot) (*BehaviorDecision, error) {
	r.arbiter.ClearUnavailable()
	if currentContext != nil {
		currentContext.Foreground = ForegroundActionState{}
	}
	return r.ReconcileCharacter(ctx, userID, characterID, currentContext)
}
