package main

import (
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"testing"
)

func TestMemoryReadsIgnorePreviousVersionRecords(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	if err := p.services.DB.Exec(`INSERT INTO memories(id,space_id,character_id,key,value,verified_status) VALUES(?,?,?,?,?,?)`, "old-memory", p.legacySpaceID, "one", "旧版本", "不应读取", "verified").Error; err != nil {
		t.Fatal(err)
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,'memory','current','one',1,?,?)`, p.ownerID, []byte(`{"key":"当前","content":{"value":"保留"}}`), "2026-10-13T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", RoleOwnerID: p.ownerID, ResourceOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3}
	for _, managed := range []bool{true, false} {
		snapshot, err := p.Snapshot(t.Context(), scope, coordination.DataQuery{Management: managed, ResourceKind: "memory", Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Resources) != 1 || snapshot.Resources[0].ID != "current" {
			t.Fatalf("old memory leaked: %+v", snapshot)
		}
	}
	scope.Coordinated, scope.RoleOwnerID, scope.ResourceOwnerID, scope.RoleID = true, "core", "core", "cloud-role"
	roles, err := p.HistoricalRoles(t.Context(), scope)
	if err != nil || len(roles) != 1 || roles[0].ID != "one" {
		t.Fatalf("roles=%+v err=%v", roles, err)
	}
	snapshot, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{Management: true, ResourceKind: "memory", HistoricalRoleID: "one"})
	if err != nil || len(snapshot.Resources) != 1 {
		t.Fatalf("current device history=%+v err=%v", snapshot, err)
	}
}
