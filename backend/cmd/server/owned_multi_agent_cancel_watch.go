package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/interaction"
)

const ownedWorkerCancellationPollInterval = 200 * time.Millisecond

func (r *ownedMultiAgentWorkerRunner) verifyWorkerParent(ctx context.Context, req interaction.WorkerRunRequest) (*interaction.InteractionRecord, error) {
	if req.ParentInteractionID == "" || req.CoordinationID == "" || req.ParentGoalID == "" || req.ParentGoalRevision <= 0 {
		return nil, errors.New("multi_agent: missing parent delegation identity")
	}
	parent, found, err := r.tracker.Get(ctx, req.ParentInteractionID)
	if err != nil {
		return nil, err
	}
	if !found || parent == nil {
		return nil, interaction.ErrInteractionNotFound
	}
	scope := parent.Scope.Normalize()
	if scope.SpaceID != req.SpaceID || scope.CharacterID != req.CharacterID || scope.ConversationID != req.ConversationID {
		return nil, fmt.Errorf("multi_agent: parent scope no longer matches worker authorization")
	}
	if !parent.CancelRequestedAt.IsZero() || parent.IsTerminal() || parent.Status == interaction.InteractionStatusPausing || parent.Status == interaction.InteractionStatusPaused {
		return nil, fmt.Errorf("multi_agent: parent is no longer authorized for child execution")
	}
	if parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		return nil, fmt.Errorf("multi_agent: parent has no durable coordination authorization")
	}
	{
		ref := parent.RecoveryDescriptor.MultiAgent
		if ref.CoordinationID != req.CoordinationID || ref.ParentGoalID != req.ParentGoalID || ref.ParentGoalRevision != req.ParentGoalRevision {
			return nil, fmt.Errorf("multi_agent: durable parent coordination identity differs from worker")
		}
		if req.OwnedScope == nil || ref.OwnedScope == nil || *ref.OwnedScope != *req.OwnedScope {
			return nil, fmt.Errorf("multi_agent: parent owner binding changed")
		}
		if ref.ExecutionCoreID != "" && (req.OwnedScope == nil || ref.ExecutionCoreID != req.OwnedScope.CoreID) {
			return nil, fmt.Errorf("multi_agent: parent executor binding changed")
		}
		if ref.ResourceOwnerID != "" && (req.OwnedScope == nil || ref.ResourceOwnerID != req.OwnedScope.ResourceOwnerID) {
			return nil, fmt.Errorf("multi_agent: parent resource owner binding changed")
		}
		if ref.WorkspaceID != req.WorkspaceID || ref.WorkspaceDeviceID != req.WorkspaceDeviceID || ref.WorkspaceRootURI != req.WorkspaceRootURI || ref.PermissionMode != req.PermissionMode {
			return nil, fmt.Errorf("multi_agent: delegated workspace or permission differs from durable plan")
		}
		if ref.ParentGoalSpaceID != "" && ref.ParentGoalSpaceID != req.SpaceID || ref.ParentGoalCharacterID != "" && ref.ParentGoalCharacterID != req.CharacterID || ref.ParentGoalConversationID != "" && ref.ParentGoalConversationID != req.ConversationID {
			return nil, fmt.Errorf("multi_agent: parent goal ownership changed")
		}
		matchedAssignment := false
		for _, assignment := range ref.AssignmentRefs {
			if assignment.AssignmentID != req.AssignmentID {
				continue
			}
			if matchedAssignment || assignment.ChildInteractionID != r.WorkerInteractionID(req) || assignment.CharacterID != req.CharacterID || assignment.Role != req.WorkerRole || assignment.Objective != req.Objective || assignment.ExpectedOutcome != req.ExpectedOutcome || assignment.Status != string(interaction.AssignmentRunning) {
				return nil, fmt.Errorf("multi_agent: worker assignment is not durably authorized")
			}
			matchedAssignment = true
		}
		if !matchedAssignment {
			return nil, fmt.Errorf("multi_agent: worker is absent from durable parent assignment plan")
		}
		if ref.CancelRequested || ref.Status != string(interaction.CoordinationRunning) && ref.Status != string(interaction.CoordinationWaiting) {
			return nil, fmt.Errorf("multi_agent: durable coordination is no longer dispatchable")
		}
		if ref.DeadlineAt != nil && !time.Now().UTC().Before(*ref.DeadlineAt) {
			return nil, fmt.Errorf("multi_agent: durable parent coordination deadline has expired")
		}
	}
	return parent, nil
}

func (r *ownedMultiAgentWorkerRunner) workerCancellationRequested(ctx context.Context, childID string, req interaction.WorkerRunRequest) (bool, error) {
	child, found, err := r.tracker.Get(ctx, childID)
	if err != nil {
		return false, err
	}
	if !found || child == nil {
		return true, interaction.ErrInteractionNotFound
	}
	if child.ID != childID || child.CausationID != req.ParentInteractionID || child.CorrelationID != req.CoordinationID {
		return true, fmt.Errorf("multi_agent: worker interaction lineage mismatch")
	}
	childScope := child.Scope.Normalize()
	if childScope.SpaceID != req.SpaceID || childScope.CharacterID != req.CharacterID || childScope.ConversationID != "multi-agent-worker-"+req.AssignmentID || childScope.RequestID != "ma-"+req.AssignmentID || childScope.Source != "multi_agent" {
		return true, fmt.Errorf("multi_agent: worker interaction scope mismatch")
	}
	if child.RecoveryDescriptor == nil || child.RecoveryDescriptor.OwnedTurn == nil || req.OwnedScope == nil || child.RecoveryDescriptor.OwnedTurn.Scope != *req.OwnedScope {
		return true, fmt.Errorf("multi_agent: persisted worker owner binding changed")
	}
	if child.Status != interaction.InteractionStatusProcessing || !child.CancelRequestedAt.IsZero() {
		return true, nil
	}
	if _, err := r.verifyWorkerParent(ctx, req); err != nil {
		return true, err
	}
	return false, nil
}

func (r *ownedMultiAgentWorkerRunner) watchWorkerCancellation(ctx context.Context, cancel context.CancelFunc, childID string, req interaction.WorkerRunRequest, done <-chan struct{}) {
	ticker := time.NewTicker(ownedWorkerCancellationPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			probeCtx, stopProbe := context.WithTimeout(ctx, time.Second)
			stop, err := r.workerCancellationRequested(probeCtx, childID, req)
			stopProbe()
			if err != nil || stop {
				cancel()
				return
			}
		}
	}
}

func (r *ownedMultiAgentWorkerRunner) settleUnverifiedWorker(childID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	record, found, err := r.tracker.Get(ctx, childID)
	if err != nil || !found || record == nil || record.IsTerminal() {
		return
	}
	_, _ = r.tracker.TransitionCAS(ctx, childID, record.StatusVersion, interaction.InteractionStatusInterrupted)
}
