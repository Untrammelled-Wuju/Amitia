package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type round44ParentHiddenTracker struct {
	interaction.InteractionTracker
	hidden atomic.Bool
}

func (t *round44ParentHiddenTracker) Get(ctx context.Context, id string) (*interaction.InteractionRecord, bool, error) {
	if id == "parent-a" && t.hidden.Load() {
		return nil, false, nil
	}
	return t.InteractionTracker.Get(ctx, id)
}

func TestRound44MissingParentRejectsReservation(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	var calls atomic.Int32
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
		calls.Add(1)
		return business.Response{Saved: true}, nil
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	req := ownedWorkerFixture()
	if _, err := runner.StartWorker(context.Background(), req); !errors.Is(err, interaction.ErrInteractionNotFound) {
		t.Fatalf("missing parent authorized child: %v", err)
	}
	if record, found, err := tracker.Get(context.Background(), runner.WorkerInteractionID(req)); err != nil || found || record != nil || calls.Load() != 0 {
		t.Fatalf("orphan reservation or execution: record=%+v found=%v err=%v calls=%d", record, found, err, calls.Load())
	}
}

func TestRound44CrossSpaceAndRevisionCannotStartWorker(t *testing.T) {
	for _, scenario := range []string{"cross_space", "wrong_role", "wrong_conversation", "wrong_goal_revision", "wrong_core", "paused_coordination", "cancelled_coordination", "wrong_owner", "wrong_workspace", "elevated_permission", "wrong_assignment_objective", "unregistered_assignment", "no_coordination_descriptor", "unknown_coordination_status", "missing_owned_scope"} {
		t.Run(scenario, func(t *testing.T) {
			tracker := interaction.NewInMemoryTracker()
			req := ownedWorkerFixture()
			scope := interaction.InteractionScope{SpaceID: req.SpaceID, CharacterID: req.CharacterID, ConversationID: req.ConversationID, Source: "web", Channel: "web", RequestID: "parent-for-" + scenario}
			ref := &interaction.MultiAgentRecoveryRef{
				CoordinationID: req.CoordinationID, ParentGoalID: req.ParentGoalID, ParentGoalRevision: req.ParentGoalRevision,
				Status: string(interaction.CoordinationRunning), OwnedScope: req.OwnedScope, WorkspaceID: req.WorkspaceID, WorkspaceDeviceID: req.WorkspaceDeviceID, WorkspaceRootURI: req.WorkspaceRootURI, PermissionMode: req.PermissionMode,
				ExecutionCoreID: req.OwnedScope.CoreID, ResourceOwnerID: req.OwnedScope.ResourceOwnerID,
				ParentGoalSpaceID: req.SpaceID, ParentGoalCharacterID: req.CharacterID, ParentGoalConversationID: req.ConversationID,
			}
			switch scenario {
			case "cross_space":
				scope.SpaceID = "other-space"
			case "wrong_role":
				scope.CharacterID = "other-role"
			case "wrong_conversation":
				scope.ConversationID = "other-conversation"
			case "wrong_goal_revision":
				ref.ParentGoalRevision++
			case "wrong_core":
				ref.ExecutionCoreID = "other-core"
			case "paused_coordination":
				ref.Status = string(interaction.CoordinationPaused)
			case "cancelled_coordination":
				ref.CancelRequested = true
			case "unknown_coordination_status":
				ref.Status = "unknown"
			case "missing_owned_scope":
				ref.OwnedScope = nil
			case "wrong_owner":
				ref.ResourceOwnerID = "other-device"
			case "wrong_workspace":
				ref.WorkspaceID = "other-workspace"
			case "elevated_permission":
				ref.PermissionMode = "request_approval"
			case "wrong_assignment_objective":
				req.Objective = "unexpected task"
			case "unregistered_assignment":
				req.AssignmentID = "unexpected-assignment"
			}
			expected := ownedWorkerFixture()
			ref.AssignmentRefs = []interaction.AssignmentRecoveryRef{{AssignmentID: expected.AssignmentID, CharacterID: req.CharacterID, Role: req.WorkerRole, Objective: expected.Objective, ExpectedOutcome: req.ExpectedOutcome, ChildInteractionID: newOwnedMultiAgentWorkerRunner(nil, tracker).WorkerInteractionID(expected), Status: string(interaction.AssignmentRunning)}}
			parent := interaction.NewInteractionRecord(scope)
			parent.ID = req.ParentInteractionID
			parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, MultiAgent: ref}
			if scenario == "no_coordination_descriptor" {
				parent.RecoveryDescriptor = nil
			}
			if err := tracker.Create(context.Background(), parent); err != nil {
				t.Fatal(err)
			}
			var runs atomic.Int32
			runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(context.Context, coordination.ExecutionScope, business.Request, []business.ProjectWorkspace, string) (business.Response, error) {
				runs.Add(1)
				return business.Response{Saved: true}, nil
			}), tracker)
			t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
			if _, err := runner.StartWorker(context.Background(), req); err == nil || runs.Load() != 0 {
				t.Fatalf("invalid parent delegated worker: err=%v runs=%d", err, runs.Load())
			}
		})
	}
}

func TestRound44ParentDisappearanceStopsActiveWorker(t *testing.T) {
	tracker := &round44ParentHiddenTracker{InteractionTracker: interaction.NewInMemoryTracker()}
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	stopped := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker never began")
	}
	tracker.hidden.Store(true)
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("orphan worker continued after parent became unavailable")
	}
	waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
}

func TestRound44ChildLineageChangeStopsActiveWorker(t *testing.T) {
	tracker := interaction.NewInMemoryTracker()
	registerOwnedWorkerParent(t, tracker)
	started := make(chan struct{})
	stopped := make(chan struct{})
	runner := newOwnedMultiAgentWorkerRunner(ownedWorkerEngineFunc(func(ctx context.Context, _ coordination.ExecutionScope, _ business.Request, _ []business.ProjectWorkspace, _ string) (business.Response, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return business.Response{}, ctx.Err()
	}), tracker)
	t.Cleanup(func() { _ = runner.Shutdown(context.Background()) })
	childID, err := runner.StartWorker(context.Background(), ownedWorkerFixture())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker never began")
	}
	alteredParent := "unrelated-parent"
	if _, err := tracker.UpdateMetadata(context.Background(), childID, interaction.InteractionMetadataUpdate{CausationID: &alteredParent}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("worker continued after lineage changed")
	}
	waitOwnedWorkerRecord(t, tracker, childID, interaction.InteractionStatusInterrupted)
}
