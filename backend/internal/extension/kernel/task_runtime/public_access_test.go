package task_runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestPublicTaskAccessRequiresRelatedDeviceOrCurrentAdministrator(t *testing.T) {
	for _, scenario := range []string{"initiator", "target", "administrator", "foreign_device", "foreign_space", "legacy", "legacy_admin", "missing_actor", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			service, _, authority, run, definition := taskAuthorityFixture(t)
			service.store = &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "caller"}
			if scenario == "target" {
				actor.DeviceID = "target"
			}
			if scenario == "administrator" || scenario == "legacy_admin" {
				actor.DeviceID, actor.Permissions = "admin", []string{auth.PermSystemAdmin}
			}
			if scenario == "foreign_device" {
				actor.DeviceID = "other"
			}
			if scenario == "foreign_space" {
				actor.SpaceID = "other"
			}
			if scenario == "legacy" || scenario == "legacy_admin" {
				service.store.(*ownedCallbackStore).run.ScopeSnapshotID = ""
			}
			active := true
			service.config.PublicRequestGuard = func(ctx context.Context, _ *coordination.ExecutionScope) (context.Context, func(), error) {
				if scenario == "revoked" {
					return ctx, nil, coordination.ErrScopeExpired
				}
				return coordination.WithAdditionalGuard(ctx, func(context.Context) error {
					if !active {
						return coordination.ErrScopeExpired
					}
					return nil
				}), func() {}, nil
			}
			service.config.OwnedExecutionGuard = func(ctx context.Context, expected coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
				return coordination.WithScope(ctx, expected), func() {}, nil
			}
			ctx := auth.WithActor(t.Context(), actor)
			if scenario == "missing_actor" {
				ctx = t.Context()
			}
			current, finish, err := service.OpenPublicTaskRequest(ctx, run.TaskRunID)
			allowed := scenario == "initiator" || scenario == "target" || scenario == "administrator" || scenario == "legacy_admin"
			if allowed != (err == nil) {
				t.Fatalf("任务权限判断错误: %v", err)
			}
			if err != nil {
				return
			}
			defer finish()
			if scenario != "legacy_admin" {
				actual, owned := coordination.FromContext(current)
				if !owned || actual != authority {
					t.Fatal("任务读取未保留原任务的数据归属授权")
				}
			}
			active = false
			if coordination.ValidateCurrent(current) == nil {
				t.Fatal("管理员或设备权限撤销后任务请求仍然有效")
			}
		})
	}
}

func TestPublicTaskHistoryReadsExpiredScopeWithoutRestoringExecution(t *testing.T) {
	service, snapshots, saved, run, definition := taskAuthorityFixture(t)
	run.DefinitionFingerprint = strings.Repeat("a", 64)
	snapshot, err := snapshots.GetSnapshot(t.Context(), run.ScopeSnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Hour)
	snapshot.SnapshotID, snapshot.ExpiresAt = "historical-expired", &expired
	if err := snapshots.SaveSnapshot(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	run.ScopeSnapshotID = snapshot.SnapshotID
	service.store = &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
	active := true
	service.config.PublicRequestGuard = func(ctx context.Context, _ *coordination.ExecutionScope) (context.Context, func(), error) {
		return coordination.WithAdditionalGuard(ctx, func(context.Context) error {
			if !active {
				return coordination.ErrScopeExpired
			}
			return nil
		}), func() {}, nil
	}
	service.config.OwnedReadGuard = func(ctx context.Context, original coordination.ExecutionScope, task *TaskRun) (context.Context, func(), error) {
		current := original
		current.Coordinated, current.ModeRevision, current.PermissionRevision, current.TargetPermissionRevision = true, original.ModeRevision+1, original.PermissionRevision+1, original.TargetPermissionRevision+1
		current.RoleOwnerID, current.ResourceOwnerID, current.RoleID, current.RoleRevision = "core", "core", "", 0
		read, err := coordination.WithTaskReadAuthority(coordination.WithScope(ctx, current), current, coordination.TaskReadProof{Scope: original, TaskRunID: task.TaskRunID, DefinitionFingerprint: task.DefinitionFingerprint})
		return read, func() {}, err
	}
	ctx := auth.WithActor(t.Context(), &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "target"})
	read, finish, err := service.OpenPublicTaskRead(ctx, run.TaskRunID)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if actual, ok := coordination.FromContext(read); !ok || actual != saved {
		t.Fatal("已过期任务的历史读取改变了数据归属")
	}
	if err := service.validateTaskReadScope(read, run); err != nil {
		t.Fatalf("当前授权不能读取过期任务的历史: %v", err)
	}
	if _, _, err := service.restoreTaskAuthority(read, run); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("只读历史上下文恢复了旧执行: %v", err)
	}
	if _, _, err := service.OpenPublicTaskRequest(ctx, run.TaskRunID); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("任务管理复用了过期的执行快照: %v", err)
	}
	active = false
	if coordination.ValidateCurrent(read) == nil {
		t.Fatal("读取权限失效后历史上下文仍有效")
	}
}
