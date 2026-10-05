package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func taskAuthorityFixture(t *testing.T) (*TaskRuntimeService, *scope.MemoryScopeStore, coordination.ExecutionScope, *TaskRun, *TaskDefinition) {
	t.Helper()
	authority := coordination.ExecutionScope{SpaceID: "core", AuthorizationRealm: "core", CoreID: "core", InitiatorDeviceID: "caller", TargetDeviceID: "target", RoleID: "role", RoleRevision: 7, ResourceOwnerID: "target", RoleOwnerID: "target", ProviderEpoch: 2, TargetProviderEpoch: 3, ModeRevision: 4, PermissionRevision: 5, TargetPermissionRevision: 6, RequestID: "request", ExecutionID: "execution", TurnID: "turn"}
	encoded, err := json.Marshal(authority)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := scope.NewMemoryScopeStore()
	if err := snapshots.SaveSnapshot(t.Context(), scope.ScopeSnapshot{SnapshotID: "snapshot", SpaceID: "core", InvocationID: "invocation", ExtensionID: "extension", ModuleID: "module", CharacterID: "role", OwnedExecutionScope: encoded}); err != nil {
		t.Fatal(err)
	}
	config := DefaultTaskRuntimeConfig()
	config.AuthoritySnapshots = snapshots
	service := NewTaskRuntimeService(nil, config)
	run := &TaskRun{TaskRunID: "run", ScopeSnapshotID: "snapshot", InvocationID: "invocation", ExtensionID: "extension", ModuleID: "module"}
	definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ModuleID: "module"}
	return service, snapshots, authority, run, definition
}

func TestOwnedTaskEnqueueRejectsAuthorityLossBeforeStoringInput(t *testing.T) {
	service, _, authority, run, definition := taskAuthorityFixture(t)
	req := EnqueueTaskRequest{ScopeSnapshotID: run.ScopeSnapshotID, InvocationID: run.InvocationID, Input: json.RawMessage(`{"private":"device-owned"}`)}
	ctx := coordination.WithScope(t.Context(), authority)
	if _, err := service.Enqueue(ctx, req, definition); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("missing ownership executor was accepted: %v", err)
	}
	service.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	if err := service.validateEnqueueAuthority(ctx, req, definition); err != nil {
		t.Fatal(err)
	}
	if err := service.validateEnqueueAuthority(t.Context(), req, definition); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("detached snapshot was accepted: %v", err)
	}
	changed := authority
	changed.ResourceOwnerID = "other"
	if err := service.validateEnqueueAuthority(coordination.WithScope(t.Context(), changed), req, definition); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("different owner was accepted: %v", err)
	}
	req.ScopeSnapshotID = ""
	if err := service.validateEnqueueAuthority(ctx, req, definition); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("owned execution lost its snapshot: %v", err)
	}
}

func TestTaskReadRejectsExpiredDetachedAndReplacedPersistentScope(t *testing.T) {
	service, snapshots, authority, run, _ := taskAuthorityFixture(t)
	ctx := coordination.WithScope(t.Context(), authority)
	if err := service.validateTaskReadScope(ctx, run); err != nil {
		t.Fatal(err)
	}
	changed := CloneTaskRun(run)
	changed.ScopeSnapshotID = ""
	if err := service.validateTaskReadScope(ctx, changed); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("读取设备任务时授权降级: %v", err)
	}
	snapshot, err := snapshots.GetSnapshot(t.Context(), run.ScopeSnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Second)
	snapshot.SnapshotID = "expired-snapshot"
	snapshot.ExpiresAt = &expired
	if err := snapshots.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	changed.ScopeSnapshotID = snapshot.SnapshotID
	if err := service.validateTaskReadScope(ctx, changed); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("过期任务快照仍允许读取: %v", err)
	}
	snapshot.ExpiresAt = nil
	snapshot.SnapshotID = "replacement-snapshot"
	replacement := authority
	replacement.PermissionRevision++
	snapshot.OwnedExecutionScope, _ = json.Marshal(replacement)
	if err := snapshots.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	changed.ScopeSnapshotID = snapshot.SnapshotID
	if err := service.validateTaskReadScope(ctx, changed); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("已替换快照借用旧范围读取: %v", err)
	}
}

func TestOwnedTaskQueueRestoresExactScopeAndRejectsChangedAuthority(t *testing.T) {
	service, snapshots, authority, run, _ := taskAuthorityFixture(t)
	finished := 0
	service.config.OwnedExecutionGuard = func(ctx context.Context, expected coordination.ExecutionScope, saved *TaskRun) (context.Context, func(), error) {
		if expected != authority || saved.TaskRunID != run.TaskRunID {
			t.Fatal("queue changed stored authority")
		}
		return coordination.WithAdditionalGuard(coordination.WithScope(ctx, expected), func(context.Context) error { return nil }), func() { finished++ }, nil
	}
	restored, finish, err := service.restoreTaskAuthority(t.Context(), run)
	if err != nil {
		t.Fatal(err)
	}
	actual, ok := coordination.FromContext(restored)
	if !ok || actual != authority {
		t.Fatal("queue did not restore complete authority")
	}
	finish()
	if finished != 1 {
		t.Fatal("restored authority was not released")
	}
	service.config.OwnedExecutionGuard = func(ctx context.Context, expected coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		expected.CoreID = "replacement"
		return coordination.WithScope(ctx, expected), func() { finished++ }, nil
	}
	if _, _, err := service.restoreTaskAuthority(t.Context(), run); !IsTaskErrorCode(err, ErrTaskScopeDenied) || finished != 2 {
		t.Fatalf("queue accepted replaced core or leaked authority: %v", err)
	}
	service.config.OwnedExecutionGuard = func(ctx context.Context, expected coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithAdditionalGuard(coordination.WithScope(ctx, expected), func(context.Context) error { return coordination.ErrScopeExpired }), func() { finished++ }, nil
	}
	if _, _, err := service.restoreTaskAuthority(t.Context(), run); !errors.Is(err, coordination.ErrScopeExpired) || finished != 3 {
		t.Fatalf("queue accepted expired policy or leaked authority: %v", err)
	}
	if err := snapshots.DeleteSnapshot(t.Context(), run.ScopeSnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.restoreTaskAuthority(t.Context(), run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("queue executed without persisted authority: %v", err)
	}
}

func TestTaskQueueRejectsExpiredAndMismatchedSnapshots(t *testing.T) {
	service, snapshots, _, run, _ := taskAuthorityFixture(t)
	for _, change := range []func(*TaskRun){
		func(r *TaskRun) { r.InvocationID = "other" },
		func(r *TaskRun) { r.ExtensionID = "other" },
		func(r *TaskRun) { r.ModuleID = "other" },
	} {
		copy := *run
		change(&copy)
		if _, _, err := service.restoreTaskAuthority(t.Context(), &copy); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
			t.Fatalf("mismatched snapshot was accepted: %v", err)
		}
	}
	expired := time.Now().Add(-time.Minute)
	if err := snapshots.SaveSnapshot(t.Context(), scope.ScopeSnapshot{SnapshotID: "expired", InvocationID: run.InvocationID, ExtensionID: run.ExtensionID, ModuleID: run.ModuleID, ExpiresAt: &expired}); err != nil {
		t.Fatal(err)
	}
	run.ScopeSnapshotID = "expired"
	if _, _, err := service.restoreTaskAuthority(t.Context(), run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("expired legacy authorization was accepted: %v", err)
	}
}
