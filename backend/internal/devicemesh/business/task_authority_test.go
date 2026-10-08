package business

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestTaskExecutionAuthorityRequiresTaskGrantRoleAndCurrentMode(t *testing.T) {
	engine, db, service, model := engineHarness(t)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_devices(device_id,space_id,trust_state,created_at,last_seen_at) VALUES('b','core','trusted','now','now')`); err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", CoreID: "core", DeviceID: "a", TargetDeviceID: "b", RequestID: "task-request"}
	if _, _, _, err := engine.TaskRoles(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("角色列表绕过任务能力授权: %v", err)
	}
	if _, _, finish, err := engine.OpenTaskExecution(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		if finish != nil {
			finish()
		}
		t.Fatalf("跨设备任务未要求目标授予任务能力: %v", err)
	}
	if _, err := service.SetCapabilityGrant(t.Context(), "core", "a", "b", "ai.chat", 0, true); err != nil {
		t.Fatal(err)
	}
	if _, _, finish, err := engine.OpenTaskExecution(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		if finish != nil {
			finish()
		}
		t.Fatalf("聊天权限被当作设备任务执行权限: %v", err)
	}
	grant, err := service.SetCapabilityGrant(t.Context(), "core", "a", "b", "task.execute", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	roles, roleScope, _, err := engine.TaskRoles(t.Context(), request)
	if err != nil || len(roles) != 1 || roles[0].ID != "role" || roleScope.RoleOwnerID != "b" || model.calls.Load() != 0 {
		t.Fatalf("任务角色列表未使用目标设备归属: %+v %+v %v", roles, roleScope, err)
	}
	current, authority, finish, err := engine.OpenTaskExecution(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if authority.ResourceOwnerID != "b" || authority.RoleID != "role" || authority.RoleRevision != 1 || authority.ExecutionID == "" || authority.TurnID == "" || model.calls.Load() != 0 {
		t.Fatalf("任务授权未使用目标设备角色与归属: %+v", authority)
	}
	for _, field := range []string{"permission", "epoch", "owner", "caller", "realm"} {
		expected := authority
		switch field {
		case "permission":
			expected.TargetPermissionRevision++
		case "epoch":
			expected.ProviderEpoch++
		case "owner":
			expected.ResourceOwnerID = "other"
		case "caller":
			expected.InitiatorDeviceID = "other"
		case "realm":
			expected.AuthorizationRealm = "other"
		}
		stale := request
		stale.ExpectedScope = &expected
		if _, _, closeStale, err := engine.OpenTaskExecution(t.Context(), stale); !errors.Is(err, coordination.ErrScopeExpired) {
			if closeStale != nil {
				closeStale()
			}
			t.Fatalf("任务接受了过期的 %s 快照: %v", field, err)
		}
	}
	confirmed := request
	confirmed.ExpectedScope = &authority
	if _, _, closeConfirmed, err := engine.OpenTaskExecution(t.Context(), confirmed); err != nil {
		t.Fatalf("有效任务权限快照被拒绝: %v", err)
	} else {
		closeConfirmed()
	}
	again, same, closeAgain, err := engine.OpenTaskExecution(t.Context(), request)
	if err != nil || same != authority || coordination.ValidateCurrent(again) != nil {
		t.Fatalf("相同任务请求的执行范围不稳定: %+v %v", same, err)
	}
	closeAgain()
	if _, err := service.SetCapabilityGrant(t.Context(), "core", "a", "b", "task.execute", grant.Revision, false); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateCurrent(current); err == nil || context.Cause(current) == nil {
		t.Fatal("撤销任务能力后执行范围仍然有效")
	}
	if _, _, _, err := engine.TaskRoles(t.Context(), request); !errors.Is(err, coordination.ErrCapabilityGrant) {
		t.Fatalf("撤销任务权限后仍可读取目标角色: %v", err)
	}
	request.TargetDeviceID, request.RequestID = "a", "self-request"
	self, original, closeSelf, err := engine.OpenTaskExecution(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSelf()
	policy, err := service.ChangeMode(t.Context(), "core", "a", original.ModeRevision, true, "role")
	if err != nil {
		t.Fatal(err)
	}
	if coordination.ValidateCurrent(self) == nil {
		t.Fatal("切换统筹模式后原任务授权没有失效")
	}
	coordinated, owned, closeCoordinated, err := engine.OpenTaskExecution(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeCoordinated()
	if !owned.Coordinated || owned.ResourceOwnerID != "core" || owned.RoleOwnerID != "core" || owned.ModeRevision != policy.ModeRevision || coordination.ValidateCurrent(coordinated) != nil {
		t.Fatalf("统筹任务未使用 Core 角色与数据归属: %+v", owned)
	}
	request.RoleID = "missing-role"
	if _, _, closeMissing, err := engine.OpenTaskExecution(t.Context(), request); !errors.Is(err, coordination.ErrRoleRequired) {
		if closeMissing != nil {
			closeMissing()
		}
		t.Fatalf("无效角色被自动替换: %v", err)
	}
}
