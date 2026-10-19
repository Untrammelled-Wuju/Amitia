package main

import (
	"context"
	"errors"
	"sync"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type parentTurnRecoveryRuntime struct{ mu sync.Mutex }

func newParentTurnRecoveryRuntime() *parentTurnRecoveryRuntime { return &parentTurnRecoveryRuntime{} }

type ownedTurnReconciler interface {
	ReconcileOwnedTurn(context.Context, coordination.ExecutionScope, string) (business.OwnedTurnReconciliation, error)
}

func (r *parentTurnRecoveryRuntime) Scan(ctx, parentCtx context.Context, svc *AppServices) error {
	if r == nil || svc == nil || svc.OwnedBusiness == nil || svc.WorkbenchTaskTracker == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return reconcileOwnedParents(ctx, svc.WorkbenchTaskTracker, svc.OwnedBusiness)
}

func reconcileOwnedParents(ctx context.Context, tracker interaction.InteractionTracker, engine ownedTurnReconciler) error {
	var failures []error
	err := tracker.Range(ctx, func(record *interaction.InteractionRecord) bool {
		if ctx.Err() != nil {
			return false
		}
		if record == nil || record.IsTerminal() || record.CausationID != "" || record.RecoveryDescriptor == nil || record.RecoveryDescriptor.OwnedTurn == nil {
			return true
		}
		descriptor := *record.RecoveryDescriptor
		fingerprint := descriptor.Fingerprint
		descriptor.ComputeFingerprint()
		ref := descriptor.OwnedTurn
		if fingerprint == "" || descriptor.Fingerprint != fingerprint || ref.InputFingerprint == "" || ref.RequestID == "" || ref.RequestID != ref.Scope.RequestID || record.Scope.SpaceID != ref.Scope.SpaceID || record.Scope.CharacterID != ref.Scope.RoleID || descriptor.Scope.ConversationID != record.Scope.ConversationID || descriptor.Interaction.InteractionID != record.ID {
			failures = append(failures, coordination.ErrWrongOwner)
			return true
		}
		result, err := engine.ReconcileOwnedTurn(ctx, ref.Scope, record.Scope.ConversationID)
		if err != nil && !errors.Is(err, business.ErrUncertainExecution) {
			failures = append(failures, err)
			return true
		}
		if result.Status == "running" {
			return true
		}
		latest, found, err := tracker.Get(ctx, record.ID)
		if err != nil {
			failures = append(failures, err)
			return true
		}
		if !found || latest == nil || latest.IsTerminal() {
			return true
		}
		if result.Status != "completed" {
			descriptor.State = interaction.RecoveryDescriptorManualIntervention
			descriptor.ComputeFingerprint()
			marker := "needs_reconciliation/" + ref.RequestID
			_, err = tracker.UpdateMetadata(ctx, latest.ID, interaction.InteractionMetadataUpdate{ExpectedStatusVersion: &latest.StatusVersion, RecoveryDescriptor: &descriptor, ResultRef: &marker})
		} else if descriptor.MultiAgent != nil {
			marker := interaction.OwnedReplyFinishedPrefix + ref.RequestID
			_, err = tracker.UpdateMetadata(ctx, latest.ID, interaction.InteractionMetadataUpdate{ExpectedStatusVersion: &latest.StatusVersion, ResultRef: &marker})
			if err == nil {
				err = interaction.FinalizeOwnedParent(ctx, tracker, latest.ID)
			}
		} else {
			err = completeRecoveredOwnedParent(ctx, tracker, latest, "owned-turn/"+ref.RequestID)
		}
		if err != nil {
			failures = append(failures, err)
		}
		return true
	})
	return errors.Join(append(failures, err)...)
}

func completeRecoveredOwnedParent(ctx context.Context, tracker interaction.InteractionTracker, parent *interaction.InteractionRecord, resultRef string) error {
	var err error
	for {
		switch parent.Status {
		case interaction.InteractionStatusReceived, interaction.InteractionStatusNormalized, interaction.InteractionStatusQueued:
			parent, err = tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, interaction.InteractionStatusProcessing)
		case interaction.InteractionStatusContextReady, interaction.InteractionStatusDecided:
			parent, err = tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, interaction.InteractionStatusGenerated)
		case interaction.InteractionStatusGenerated:
			parent, err = tracker.TransitionCAS(ctx, parent.ID, parent.StatusVersion, interaction.InteractionStatusCommitted)
		default:
			_, err = tracker.Complete(ctx, parent.ID, parent.StatusVersion, resultRef)
			return err
		}
		if err != nil {
			return err
		}
	}
}

func (r *parentTurnRecoveryRuntime) Wait(ctx context.Context) error { return ctx.Err() }
