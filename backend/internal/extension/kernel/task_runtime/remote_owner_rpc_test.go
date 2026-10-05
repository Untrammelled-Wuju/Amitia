package task_runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestRemoteTaskOwnerRPCRejectsForeignExpiredAndChangedExecution(t *testing.T) {
	for _, scenario := range []string{"valid", "checkpoint", "foreign_device", "local_ui", "changed_scope", "generation", "lease", "session", "connection", "expired", "cancelled", "unsupported", "changed_during_ack", "unconfirmed"} {
		t.Run(scenario, func(t *testing.T) {
			service, snapshots, authority, run, definition := taskAuthorityFixture(t)
			authority.Coordinated = true
			authority.ResourceOwnerID, authority.RoleOwnerID = "core", "core"
			raw, _ := json.Marshal(authority)
			run.ScopeSnapshotID = "core-owner-snapshot"
			if err := snapshots.SaveSnapshot(t.Context(), scope.ScopeSnapshot{SnapshotID: run.ScopeSnapshotID, SpaceID: authority.SpaceID, InvocationID: run.InvocationID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, CharacterID: authority.RoleID, OwnedExecutionScope: raw}); err != nil {
				t.Fatal(err)
			}
			run.TaskDefinitionID = definition.TaskID
			run.DefinitionFingerprint, _ = taskDefinitionFingerprint(definition)
			run.Generation, run.ExecutionAttemptID, run.Status = 1, "attempt", RunStatusRunning
			run.ExecutionPlacement = TaskExecutionPlacementDevice
			run.ExecutionTarget = TaskExecutionTarget{SpaceID: "core", DeviceID: "target", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 7}
			run.Input = json.RawMessage(`{"private":"core-only-input"}`)
			run.InputHash = hashBytes(run.Input)
			data := &taskInputData{}
			if err := (AcknowledgedTaskInputPort{Data: data}).SaveInput(coordination.WithScope(t.Context(), authority), run); err != nil {
				t.Fatal(err)
			}
			run.Input = nil
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			service.store = store
			service.config.OwnedStorage = AcknowledgedTaskStoragePort{Data: data}
			service.config.OwnedCheckpoints = AcknowledgedTaskCheckpointPort{Data: data}
			service.config.OwnedExecutionGuard = func(ctx context.Context, expected coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
				return coordination.WithScope(ctx, expected), func() {}, nil
			}
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "target", RuntimeID: "runtime"}
			request := RemoteTaskOwnerRequest{Scope: authority, TaskGeneration: 1, AttemptID: "attempt", LeaseID: "lease", SessionID: "session", ConnectionGeneration: 7, AuthorityCallID: "root-authority", RequestID: "host-nonce/request", Method: "task.storage.set", Params: json.RawMessage(`{"task_run_id":"run","key":"cursor","value":{"private":"core-only-value"}}`)}
			manager := NewPendingTaskManager()
			pending, err := manager.Register(TaskExecutionRequest{Run: run, AttemptID: run.ExecutionAttemptID, Target: run.ExecutionTarget}, "session", 7, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Cancel(run.TaskRunID, "cleanup")
			request.LeaseID = pending.LeaseID
			if err := manager.BindAuthority(run.TaskRunID, "attempt", "session", 7, func(context.Context) error { return nil }, "root-authority"); err != nil {
				t.Fatal(err)
			}
			if !manager.ClaimBound(run.TaskRunID, "attempt", request.LeaseID, "session", 7, "worker", time.Minute) {
				t.Fatal("claim failed")
			}
			binding := true
			check := func() error {
				if !binding || !manager.ValidateOwnerBound(run.TaskRunID, request.AttemptID, request.LeaseID, request.SessionID, request.ConnectionGeneration, request.AuthorityCallID) {
					return coordination.ErrScopeExpired
				}
				return nil
			}
			switch scenario {
			case "checkpoint":
				request.Method = "task.checkpoint.save"
				request.Params = json.RawMessage(`{"task_run_id":"run","version":1,"payload":{"cursor":1,"data":{"private":"core-checkpoint"}}}`)
			case "foreign_device":
				actor.DeviceID = "other"
			case "local_ui":
				actor.PrincipalType = auth.PrincipalLocalUI
			case "changed_scope":
				request.Scope.RoleRevision++
			case "generation":
				request.TaskGeneration++
			case "lease":
				request.LeaseID = "other"
			case "session":
				request.SessionID = "old"
			case "connection":
				request.ConnectionGeneration++
			case "expired":
				manager.mu.Lock()
				pending.LeaseExpiresAt = time.Now().Add(-time.Second)
				manager.mu.Unlock()
			case "cancelled":
				store.run.Status = RunStatusCancelled
			case "unsupported":
				request.Method = "task.arbitrary.resource"
			case "changed_during_ack":
				data.afterCommit = func() { binding = false; store.run.Status = RunStatusCancelled }
			case "unconfirmed":
				data.wrongAck = true
			}
			result, err := service.CallRemoteOwner(auth.WithActor(t.Context(), actor), run.TaskRunID, request, check)
			valid := scenario == "valid" || scenario == "checkpoint"
			if valid && (err != nil || !json.Valid(result)) || !valid && err == nil {
				t.Fatalf("owner RPC scenario=%s result=%s err=%v", scenario, result, err)
			}
			if scenario == "valid" {
				request.Method, request.RequestID, request.Params = "task.storage.get", "read-request", json.RawMessage(`{"task_run_id":"run","key":"cursor"}`)
				result, err = service.CallRemoteOwner(auth.WithActor(t.Context(), actor), run.TaskRunID, request, check)
				if err != nil || string(result) != `{"value":{"private":"core-only-value"}}` {
					t.Fatalf("Core same owner storage unreadable: %s %v", result, err)
				}
			}
		})
	}
}

func TestRemoteOwnerBindingRequiresClaimAndExactAuthorityCall(t *testing.T) {
	manager := NewPendingTaskManager()
	request := TaskExecutionRequest{Run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task"}, AttemptID: "attempt"}
	pt, err := manager.Register(request, "session", 7, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("run", "cleanup")
	confirm := func(context.Context) error { return nil }
	if err := manager.BindAuthority("run", "attempt", "session", 7, confirm, ""); err == nil {
		t.Fatal("empty authority id accepted")
	}
	if err := manager.BindAuthority("run", "attempt", "session", 7, confirm, "root-call"); err != nil {
		t.Fatal(err)
	}
	if manager.ValidateOwnerBound("run", "attempt", pt.LeaseID, "session", 7, "root-call") {
		t.Fatal("unclaimed owner call accepted")
	}
	if !manager.ClaimBound("run", "attempt", pt.LeaseID, "session", 7, "worker", time.Minute) {
		t.Fatal("claim failed")
	}
	if !manager.ValidateOwnerBound("run", "attempt", pt.LeaseID, "session", 7, "root-call") || manager.ValidateOwnerBound("run", "attempt", pt.LeaseID, "session", 7, "other-call") {
		t.Fatal("owner authority binding mismatch")
	}
}
