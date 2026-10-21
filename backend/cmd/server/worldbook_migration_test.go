package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestLegacyWorldbookMigrationIsOwnerScopedAndIdempotent(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	rows := []map[string]any{
		{"id": "global", "space_id": p.legacySpaceID, "match_type": "keyword", "match_pattern": "联盟", "match_scope": "full_context", "inject_content": "全局规则", "priority": 1, "character_id": "", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"},
		{"id": "role", "space_id": p.legacySpaceID, "match_type": "exact", "match_pattern": "晨星", "match_scope": "user_message", "inject_content": "角色规则", "priority": 2, "character_id": "one", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-02T00:00:00Z"},
	}
	for _, row := range rows {
		if err := p.services.DB.Table("world_book").Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	kernelDB := p.services.KernelContainer.DeviceRegistry.Database()
	owner := "device-a"
	for i := 0; i < 2; i++ {
		if err := migrateLegacyWorldbook(t.Context(), p.services.DB, kernelDB, p.legacySpaceID, owner); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := kernelDB.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0`, owner).Scan(&count); err != nil || count != 2 {
		t.Fatalf("migrated worldbook rows=%d err=%v", count, err)
	}
	resourceRows, err := kernelDB.Query(`SELECT role_id,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0 ORDER BY role_id`, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer resourceRows.Close()
	roles := map[string]bool{}
	for resourceRows.Next() {
		var role string
		var raw []byte
		if err := resourceRows.Scan(&role, &raw); err != nil {
			t.Fatal(err)
		}
		var rule business.WorldbookRule
		if json.Unmarshal(raw, &rule) != nil || rule.ID == "" {
			t.Fatal("migrated worldbook body invalid")
		}
		roles[role] = true
	}
	if !roles[coordination.GlobalWorldbookRole] || !roles["one"] {
		t.Fatalf("migrated worldbook partitions missing: %+v", roles)
	}
	if err := migrateLegacyWorldbook(t.Context(), p.services.DB, kernelDB, p.legacySpaceID, "core-b"); err != nil {
		t.Fatal(err)
	}
	if err := kernelDB.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='worldbook' AND deleted=0`, "core-b").Scan(&count); err != nil || count != 2 {
		t.Fatalf("independent owner migration rows=%d err=%v", count, err)
	}
}
