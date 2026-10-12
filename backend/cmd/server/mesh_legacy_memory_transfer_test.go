package main

import (
	"encoding/json"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/memory"
	"testing"
)

func TestMemoryScopeLegacyTransferCASReplayAndDemote(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	row := memory.Memory{ID: "old-private", SpaceID: p.legacySpaceID, CharacterID: "one", Scope: "character", Key: "偏好", Value: "茶", Version: 1}
	if err := p.services.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	expected, _ := json.Marshal(row)
	scope := coordination.ExecutionScope{ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, MemoryRoleID: "one", RequestID: "promote"}
	input := coordination.LegacyMemoryTransferRequest{Kind: "memory", ID: row.ID, ExpectedRecord: expected, TargetRoleID: coordination.GlobalMemoryRole}
	ack, err := p.TransferLegacyMemory(t.Context(), scope, input)
	if err != nil || ack.Versions["memory/legacy/"+row.ID] != 1 {
		t.Fatalf("promote %+v %v", ack, err)
	}
	if _, err := p.TransferLegacyMemory(t.Context(), scope, input); err != nil {
		t.Fatalf("replay %v", err)
	}
	if err := p.services.DB.Where("id=?", row.ID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Scope != "user" || row.Value != "茶" || row.Version != 2 {
		t.Fatalf("moved %+v", row)
	}
	scope.MemoryRoleID = coordination.GlobalMemoryRole
	scope.RequestID = "stale-demote"
	input.TargetRoleID = "two"
	if _, err := p.TransferLegacyMemory(t.Context(), scope, input); err == nil {
		t.Fatal("stale legacy snapshot accepted")
	}
	scope.RequestID = "demote"
	input.ExpectedRecord, _ = json.Marshal(row)
	if _, err := p.TransferLegacyMemory(t.Context(), scope, input); err != nil {
		t.Fatal(err)
	}
	if err := p.services.DB.Where("id=?", row.ID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Scope != "character" || row.CharacterID != "two" || row.Version != 3 || row.Value != "茶" {
		t.Fatalf("demoted %+v", row)
	}
	var receipts int64
	p.services.DB.Table("memory_scope_transfers").Count(&receipts)
	if receipts != 2 {
		t.Fatalf("migration audit=%d", receipts)
	}
}
