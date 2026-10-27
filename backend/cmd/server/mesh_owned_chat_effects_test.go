package main

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedMoodPsycheResourceUsesRoleQualifiedState(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", RoleOwnerID: p.ownerID, ResourceOwnerID: p.ownerID, RoleID: "two", RoleRevision: 1}
	insert := func(id, role string, deleted int) {
		t.Helper()
		_, err := p.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,'psyche',?,?,1,?,?,?)`, p.ownerID, id, role, deleted, []byte(`{}`), time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			t.Fatal(err)
		}
	}
	clear := func() {
		t.Helper()
		if _, err := p.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `DELETE FROM kernel_device_owned_resources WHERE owner_id=? AND kind='psyche'`, p.ownerID); err != nil {
			t.Fatal(err)
		}
	}

	insert("role", "one", 0)
	resource, err := p.ownedMoodPsycheResource(t.Context(), scope)
	if err != nil || resource != nil {
		t.Fatalf("其他角色的旧版状态泄漏到当前角色: %+v %v", resource, err)
	}

	insert("role/two", "two", 0)
	resource, err = p.ownedMoodPsycheResource(t.Context(), scope)
	if err != nil || resource == nil || resource.ID != "role/two" || resource.RoleID != "two" {
		t.Fatalf("角色限定状态未被优先读取: %+v %v", resource, err)
	}

	clear()
	insert("role", "two", 0)
	resource, err = p.ownedMoodPsycheResource(t.Context(), scope)
	if err != nil || resource == nil || resource.ID != "role" || resource.RoleID != "two" {
		t.Fatalf("同角色旧版状态未兼容读取: %+v %v", resource, err)
	}
}
