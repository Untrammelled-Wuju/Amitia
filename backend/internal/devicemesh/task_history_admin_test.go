package devicemesh_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCurrentAdministratorReadsCoreTaskAfterOriginalDeviceRevoked(t *testing.T) {
	db := meshAuthorityDB(t)
	registry := host_registry.NewRegistry(db)
	for _, device := range []runtimeidentity.DeviceID{"admin", "old-device", "ordinary"} {
		if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: device, Platform: runtimeidentity.PlatformWindows, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
			t.Fatal(err)
		}
	}
	rt, err := devicemesh.NewCloudRuntime(db, registry)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := rt.Coordination.ChangeMode(t.Context(), "core", "admin", 1, true, "current-role")
	if err != nil {
		t.Fatal(err)
	}
	policy, err = rt.Coordination.GrantAdministrator(t.Context(), "core", "admin", policy.PermissionRevision, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE kernel_devices SET trust_state = 'revoked' WHERE device_id = 'old-device'`); err != nil {
		t.Fatal(err)
	}
	proof := coordination.TaskReadProof{TaskRunID: "historical-run", DefinitionFingerprint: strings.Repeat("a", 64), Scope: coordination.ExecutionScope{SpaceID: "core", CoreID: "core", AuthorizationRealm: "core", InitiatorDeviceID: "old-device", TargetDeviceID: "old-device", Coordinated: true, ResourceOwnerID: "core", RoleOwnerID: "core", RoleID: "deleted-role", RoleRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, PermissionRevision: 1, TargetPermissionRevision: 1, ModeRevision: 1, RequestID: "original-request", ExecutionID: "original-execution", TurnID: "original-turn"}}
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "admin", Permissions: []string{auth.PermSystemAdmin}}
	ctx := auth.WithActor(t.Context(), actor)
	read, finish, err := rt.OpenTaskRead(ctx, proof)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	current, saved, ok := coordination.TaskReadAuthority(read)
	if !ok || saved != proof || current.TargetDeviceID != "admin" || current.ResourceOwnerID != "core" {
		t.Fatal("Core 管理员读取改变了原任务数据归属或依赖已解绑设备")
	}
	body, _ := json.Marshal(map[string]any{"executionScope": proof.Scope, "taskRunId": proof.TaskRunID, "definitionFingerprint": proof.DefinitionFingerprint})
	resource := &coordination.Resource{OwnerID: "core", RoleID: proof.Scope.RoleID, Kind: "checkpoint", ID: "task/input/" + proof.TaskRunID, Body: body}
	if err := coordination.ValidateTaskReadResource(proof, resource.Kind, resource.ID, resource); err != nil {
		t.Fatal(err)
	}
	deviceProof := proof
	deviceProof.Scope.Coordinated = false
	deviceProof.Scope.ResourceOwnerID, deviceProof.Scope.RoleOwnerID = "old-device", "old-device"
	if _, closeRead, err := rt.OpenTaskRead(ctx, deviceProof); err == nil {
		closeRead()
		t.Fatal("Core 管理员读取了已解绑设备的私有数据")
	}
	ordinary := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: "core", DeviceID: "ordinary"}
	if _, closeRead, err := rt.OpenTaskRead(auth.WithActor(t.Context(), ordinary), proof); err == nil {
		closeRead()
		t.Fatal("无关普通设备取得了 Core 历史任务读取权限")
	}
	other := proof
	other.Scope.CoreID, other.Scope.SpaceID, other.Scope.AuthorizationRealm, other.Scope.ResourceOwnerID, other.Scope.RoleOwnerID = "other-core", "other-core", "other-core", "other-core", "other-core"
	if _, closeRead, err := rt.OpenTaskRead(ctx, other); err == nil {
		closeRead()
		t.Fatal("管理员读取了其他 Core 的专属数据")
	}
	if _, err := rt.Coordination.GrantAdministrator(t.Context(), "core", "admin", policy.PermissionRevision, false); err != nil {
		t.Fatal(err)
	}
	if err := coordination.ValidateCurrent(read); err == nil {
		t.Fatal("撤销管理员后历史读取上下文仍然有效")
	}
	if _, closeRead, err := rt.OpenTaskRead(ctx, proof); err == nil {
		closeRead()
		t.Fatal("旧管理员身份绕过了当前管理员状态")
	}
}
