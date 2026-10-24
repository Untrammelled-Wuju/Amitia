package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

func TestNativeMultiAgentTaskAuthorityIsIndependentOfParentContinuityLease(t *testing.T) {
	parent := coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", ResourceOwnerID: "device", RoleOwnerID: "device", RoleID: "role", RoleRevision: 4, InitiatorDeviceID: "device", TargetDeviceID: "device", ProviderEpoch: 3, TargetProviderEpoch: 3, ModeRevision: 2, PermissionRevision: 6, TargetPermissionRevision: 6, ContinuityRoleID: "thread-partition", MemoryRoleID: "@global-memory", RequestID: "request", TurnID: "turn", ExecutionID: "execution"}
	task := ownedTaskExecutionScope(parent)
	if task.ContinuityRoleID != "" || parent.ContinuityRoleID != "thread-partition" {
		t.Fatal("task changed parent lease partition")
	}
	task.ContinuityRoleID = parent.ContinuityRoleID
	if task != parent {
		t.Fatal("task scope lost an authorization fence")
	}
}

func TestNativeMultiAgentSharedRuntimeNormalizesIdempotentArguments(t *testing.T) {
	adapter, workers, scope, _ := testMultiAgentModelRuntime(t)
	runtime := adapter
	if got := runtime.ModelTools(t.Context(), scope); len(got) != 3 {
		t.Fatalf("missing native tools: %d", len(got))
	}
	first := runtime.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["  Inspect first module  ","Validate second module"],"strategy":"parallel"}`), scope)
	if first.Error != nil {
		t.Fatal(first.Error)
	}
	again := runtime.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect first module","Validate second module"]}`), scope)
	if again.Error != nil || len(workers.started) != 2 {
		t.Fatalf("canonical retry failed: %+v starts=%d", again, len(workers.started))
	}
	conflict := runtime.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect different module","Validate second module"]}`), scope)
	if conflict.Error == nil || conflict.Error.Code != "MULTI_AGENT_DISPATCH_CONFLICT" || len(workers.started) != 2 {
		t.Fatalf("conflicting request accepted: %+v", conflict)
	}
	invalid := runtime.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["tiny"]}`), scope)
	if invalid.Error == nil || invalid.Error.Code != "INVALID_OBJECTIVES" {
		t.Fatalf("invalid retry bypassed validation: %+v", invalid)
	}
}

func TestNativeMultiAgentOwnedAuthorizationSurvivesCoordinatorRestart(t *testing.T) {
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	bound := coordination.ExecutionScope{SpaceID: scope.SpaceID, RoleID: scope.CharacterID, CoreID: "core-a", ResourceOwnerID: "owner-a", RoleOwnerID: "role-owner", AuthorizationRealm: "realm-a", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a"}
	parent, found, err := tracker.GetByRequestID(t.Context(), scope.SpaceID, scope.RequestID)
	if err != nil || !found {
		t.Fatal(err)
	}
	parent.RecoveryDescriptor = &interaction.RecoveryDescriptor{SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, Requirement: interaction.RecoveryBestEffort, Interaction: interaction.RecoveryInteractionRef{InteractionID: parent.ID}, State: interaction.RecoveryDescriptorActive}
	parent.RecoveryDescriptor.OwnedTurn = &interaction.OwnedTurnRecoveryRef{Scope: bound, RequestID: scope.RequestID, InputFingerprint: "input-a"}
	if _, err := tracker.UpdateMetadata(t.Context(), parent.ID, interaction.InteractionMetadataUpdate{RecoveryDescriptor: parent.RecoveryDescriptor}); err != nil {
		t.Fatal(err)
	}
	ctx := coordination.WithScope(t.Context(), bound)
	issued := adapter.Execute(ctx, multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect first source","Validate second source"]}`), scope)
	if issued.Error != nil {
		t.Fatal(issued.Error)
	}
	var response struct {
		CoordinationID string `json:"coordinationId"`
	}
	if err := json.Unmarshal(issued.Output, &response); err != nil {
		t.Fatal(err)
	}
	lookup, _ := json.Marshal(map[string]string{"coordination_id": response.CoordinationID})
	for _, field := range []string{"owner", "core", "roleOwner", "realm", "missing"} {
		t.Run(field, func(t *testing.T) {
			other := bound
			switch field {
			case "owner":
				other.ResourceOwnerID = "owner-b"
			case "core":
				other.CoreID = "core-b"
			case "roleOwner":
				other.RoleOwnerID = "role-owner-b"
			case "realm":
				other.AuthorizationRealm = "realm-b"
			}
			attackCtx := coordination.WithScope(t.Context(), other)
			if field == "missing" {
				attackCtx = context.Background()
			}
			if got := adapter.Execute(attackCtx, multiAgentStatusTool, lookup, scope); got.Error == nil {
				t.Fatalf("cross-owner read accepted: %+v", got)
			}
		})
	}
	otherOwned := bound
	otherOwned.ResourceOwnerID = "owner-b"
	otherScope := scope
	otherScope.RequestID = "request-b"
	otherParent := interaction.NewInteractionRecord(parent.Scope)
	otherParent.Scope.RequestID = otherScope.RequestID
	otherParent.Status = interaction.InteractionStatusProcessing
	otherParent.RecoveryDescriptor = &interaction.RecoveryDescriptor{SchemaVersion: interaction.RecoveryDescriptorSchemaVersion, Requirement: interaction.RecoveryBestEffort, Interaction: interaction.RecoveryInteractionRef{InteractionID: otherParent.ID}, State: interaction.RecoveryDescriptorActive, OwnedTurn: &interaction.OwnedTurnRecoveryRef{Scope: otherOwned, RequestID: otherScope.RequestID, InputFingerprint: "input-b"}}
	if err := tracker.Create(t.Context(), otherParent); err != nil {
		t.Fatal(err)
	}
	otherCtx := coordination.WithScope(t.Context(), otherOwned)
	if len(adapter.ModelTools(otherCtx, otherScope)) != 3 {
		t.Fatal("other owner's valid parent was rejected")
	}
	if got := adapter.Execute(otherCtx, multiAgentStatusTool, lookup, otherScope); got.Error == nil || got.Error.Code != "COORDINATION_NOT_AUTHORIZED" {
		t.Fatalf("separate valid parent crossed owner: %+v", got)
	}
	adapter.coordinator = interaction.NewMultiAgentCoordinator(tracker, adapter.goals, nil, workers, nil, interaction.DefaultMultiAgentPolicy())
	adapter.coordinator.SetOwnedExecution(func(ctx context.Context, owned coordination.ExecutionScope, run func(context.Context) error) error {
		return run(coordination.WithScope(ctx, owned))
	})
	if got := adapter.Execute(otherCtx, multiAgentStatusTool, lookup, otherScope); got.Error == nil {
		t.Fatalf("recovery crossed owner: %+v", got)
	}
	if got := adapter.Execute(ctx, multiAgentStatusTool, lookup, scope); got.Error != nil {
		t.Fatalf("owned recovery failed: %+v", got)
	}
	if len(workers.started) != 2 {
		t.Fatalf("recovery duplicated workers: %d", len(workers.started))
	}
	denied := coordination.WithAdditionalGuard(ctx, func(context.Context) error { return coordination.ErrScopeExpired })
	if got := adapter.Execute(denied, multiAgentWaitTool, lookup, scope); got.Error == nil {
		t.Fatal("revoked scope accepted")
	}
}
