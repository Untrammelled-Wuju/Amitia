package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memory"
)

func TestHistoricalMemoryManagementPagesBothDeviceStores(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", RoleOwnerID: "core", ResourceOwnerID: "core", RoleID: "cloud-role", RoleRevision: 3, Coordinated: true, ModeRevision: 2, TargetPermissionRevision: 4}
	for i := 0; i < 145; i++ {
		id := fmt.Sprintf("old-%04d", i)
		if err := p.services.DB.Create(&memory.Memory{ID: id, CharacterID: "one", SpaceID: p.legacySpaceID, Key: id, Value: "old", VerifiedStatus: "verified", UpdatedAt: "2026-10-04T01:00:00Z"}).Error; err != nil {
			t.Fatal(err)
		}
		if i < 45 {
			_, err := p.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,'memory',?,'one',1,?,?)`, p.ownerID, id, []byte(`{"content":{"value":"device"}}`), "2026-10-04T01:00:00Z")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	roles, err := p.HistoricalRoles(t.Context(), scope)
	if err != nil || len(roles) != 1 || roles[0].ID != "one" {
		t.Fatalf("historical role catalog: %+v %v", roles, err)
	}
	query := coordination.DataQuery{Management: true, ResourceKind: "memory", HistoricalRoleID: "one", Limit: 50}
	seen := map[string]bool{}
	first := ""
	for page := 0; page < 8; page++ {
		snapshot, err := p.HistoricalSnapshot(t.Context(), scope, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range snapshot.Resources {
			key := "owned/" + row.ID
			if seen[key] || row.OwnerID != p.ownerID || row.RoleID != "one" {
				t.Fatal("mixed owned memory page")
			}
			seen[key] = true
		}
		for _, raw := range snapshot.LegacyMemories {
			var row memory.Memory
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			key := "legacy/" + row.ID
			if seen[key] || row.CharacterID != "one" {
				t.Fatal("mixed legacy memory page")
			}
			seen[key] = true
		}
		query.Cursor = snapshot.NextCursors["memory"]
		query.LegacyCursor = snapshot.NextCursors["legacyMemory"]
		if first == "" {
			first = query.LegacyCursor
		}
		if query.Cursor == "" && query.LegacyCursor == "" {
			break
		}
	}
	if len(seen) != 190 {
		t.Fatalf("historical memory truncated: %d", len(seen))
	}
	query.Cursor = ""
	query.LegacyCursor = first
	scope.TargetPermissionRevision++
	if _, err := p.HistoricalSnapshot(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("old memory cursor accepted: %v", err)
	}
}

func TestUncoordinatedMemoryManagementPagesBothOriginalAndOwnedStores(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", RoleOwnerID: p.ownerID, ResourceOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, PermissionRevision: 1}
	for i := 0; i < 63; i++ {
		id := fmt.Sprintf("existing-%03d", i)
		if err := p.services.DB.Create(&memory.Memory{ID: id, SpaceID: p.legacySpaceID, CharacterID: "one", Key: id, Value: "old", VerifiedStatus: "verified", UpdatedAt: "2026-10-04T01:00:00Z"}).Error; err != nil {
			t.Fatal(err)
		}
		if i < 17 {
			if _, err := p.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,'memory',?,'one',1,?,?)`, p.ownerID, id, []byte(`{"content":{"value":"new"}}`), "2026-10-04T01:00:00Z"); err != nil {
				t.Fatal(err)
			}
		}
	}
	query := coordination.DataQuery{Management: true, ResourceKind: "memory", Limit: 20}
	seen := map[string]bool{}
	first := ""
	for page := 0; page < 6; page++ {
		snapshot, err := p.Snapshot(t.Context(), scope, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range snapshot.Resources {
			key := "owned/" + row.ID
			if seen[key] || row.OwnerID != p.ownerID {
				t.Fatal("owned memory repeated or moved")
			}
			seen[key] = true
		}
		for _, raw := range snapshot.LegacyMemories {
			var row memory.Memory
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			key := "legacy/" + row.ID
			if seen[key] || row.CharacterID != "one" {
				t.Fatal("legacy memory repeated or crossed role")
			}
			seen[key] = true
		}
		query.Cursor, query.LegacyCursor = snapshot.NextCursors["memory"], snapshot.NextCursors["legacyMemory"]
		if first == "" {
			first = query.LegacyCursor
		}
		if query.Cursor == "" && query.LegacyCursor == "" {
			break
		}
	}
	if len(seen) != 80 || first == "" {
		t.Fatalf("existing device memory truncated: %d", len(seen))
	}
	query.LegacyCursor = first
	scope.PermissionRevision++
	if _, err := p.Snapshot(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("obsolete legacy page accepted: %v", err)
	}
}

func TestHistoricalMemoryRemainsReadableAfterDeviceRoleIsRemoved(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", RoleOwnerID: "core", ResourceOwnerID: "core", RoleID: "cloud-role", RoleRevision: 3, Coordinated: true, ModeRevision: 2, TargetPermissionRevision: 4}
	if err := p.services.DB.Create(&memory.Memory{ID: "orphan-memory", CharacterID: "removed-device-role", SpaceID: p.legacySpaceID, Key: "old", Value: "old-value", VerifiedStatus: "verified"}).Error; err != nil {
		t.Fatal(err)
	}
	roles, err := p.HistoricalRoles(t.Context(), scope)
	if err != nil || len(roles) != 1 || roles[0].ID != "removed-device-role" {
		t.Fatalf("orphan role catalog: %+v %v", roles, err)
	}
	snapshot, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "new-core-conversation", HistoricalRoleID: "removed-device-role", Management: true, ResourceKind: "memory", Limit: 50})
	if err != nil || snapshot == nil || len(snapshot.LegacyMemories) != 1 || string(snapshot.Role.Profile) != "{}" || snapshot.OwnerID != p.ownerID {
		t.Fatalf("orphan memory snapshot: %+v %v", snapshot, err)
	}
	if _, err := p.HistoricalSnapshot(t.Context(), scope, coordination.DataQuery{ConversationID: "new-core-conversation", HistoricalRoleID: "unknown-role", Management: true, ResourceKind: "memory", Limit: 50}); !errors.Is(err, coordination.ErrRoleRequired) {
		t.Fatalf("unknown historical role accepted: %v", err)
	}
}
