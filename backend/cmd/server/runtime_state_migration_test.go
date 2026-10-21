package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLegacyRuntimeStateIsOwnerScopedAndIdempotent(t *testing.T) {
	appDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE psyche_states (character_id TEXT, version TEXT, state_version INTEGER, emotion TEXT, mood TEXT, stress REAL, energy REAL, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE relationship_states (id TEXT, character_id TEXT, space_id TEXT, channel TEXT, relation_type TEXT, relation_data TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE need_states (id TEXT, character_id TEXT, need_key TEXT, current_value REAL, baseline REAL, trend REAL, saturated INTEGER, created_at TEXT, updated_at TEXT)`,
	} {
		if err := appDB.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := appDB.Exec(`INSERT INTO psyche_states(character_id,version,state_version,emotion,mood,stress,energy,created_at,updated_at) VALUES('role','legacy',1,'{"valence":0.72,"arousal":0.5,"dominance":0.5}','{"moodValence":0.5,"moodArousal":0.5}',0.2,0.61,'2026-10-21T00:00:00Z','2026-10-21T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Exec(`INSERT INTO relationship_states(id,character_id,space_id,channel,relation_type,relation_data,created_at,updated_at) VALUES('rel','role','space','*','user_character','{"trust":0.7,"familiarity":0.4,"security":0.6,"tension":0.1,"repairConfidence":0.5,"boundary":0.5}','2026-10-21T00:00:00Z','2026-10-21T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := appDB.Exec(`INSERT INTO need_states(id,character_id,need_key,current_value,baseline,trend,saturated,created_at,updated_at) VALUES('need','role','connection',0.65,0.34,0,0,'2026-10-21T00:00:00Z','2026-10-21T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}
	kernelDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kernelDB.Close() })
	if err := kernelsqlite.Migrate(t.Context(), kernelDB); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrateLegacyRuntimeState(t.Context(), appDB, kernelDB, "space", "device"); err != nil {
			t.Fatal(err)
		}
	}
	owned := coordination.NewOwnershipStore(kernelDB, "device")
	psycheResource, err := owned.Get(t.Context(), coordination.RuntimeStateKindPsyche, "role")
	if err != nil || psycheResource == nil || psycheResource.RoleID != "role" || psycheResource.Revision != 1 {
		t.Fatalf("psyche migration failed: %+v %v", psycheResource, err)
	}
	var migratedPsyche map[string]any
	if err := json.Unmarshal(psycheResource.Body, &migratedPsyche); err != nil {
		t.Fatal(err)
	}
	if migratedPsyche["valence"].(float64) != 0.72 {
		t.Fatalf("psyche value lost: %+v", migratedPsyche)
	}
	relationshipResource, err := owned.Get(t.Context(), coordination.RuntimeStateKindRelationship, "user_character")
	if err != nil || relationshipResource == nil || relationshipResource.RoleID != "role" {
		t.Fatalf("relationship migration failed: %+v %v", relationshipResource, err)
	}
	needResource, err := owned.Get(t.Context(), coordination.RuntimeStateKindNeeds, "role")
	if err != nil || needResource == nil || needResource.RoleID != "role" {
		t.Fatalf("need migration failed: %+v %v", needResource, err)
	}
	var count int
	if err := kernelDB.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id='device' AND kind IN ('psyche','relationship','needs')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("migrated resources=%d", count)
	}
}

func TestMigrateLegacyRuntimeStateWritesEmptyMarker(t *testing.T) {
	appDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty-app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE psyche_states (character_id TEXT PRIMARY KEY, version TEXT, state_version INTEGER, emotion TEXT, mood TEXT, stress REAL, energy REAL, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE relationship_states (id TEXT, character_id TEXT, space_id TEXT, channel TEXT, relation_type TEXT, relation_data TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE need_states (id TEXT, character_id TEXT, need_key TEXT, current_value REAL, baseline REAL, trend REAL, saturated INTEGER, created_at TEXT, updated_at TEXT)`,
	} {
		if err := appDB.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	kernelDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "empty-kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kernelDB.Close() })
	if err := kernelsqlite.Migrate(t.Context(), kernelDB); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyRuntimeState(t.Context(), appDB, kernelDB, "space", "device"); err != nil {
		t.Fatal(err)
	}
	marker, err := coordination.NewOwnershipStore(kernelDB, "device").Get(t.Context(), "checkpoint", "runtime-state-migration/device")
	if err != nil || marker == nil {
		t.Fatalf("marker missing: %+v %v", marker, err)
	}
}
