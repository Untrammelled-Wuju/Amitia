package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLegacyAgentSkillsIntoKernel(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:agent-skill-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE extension_agent_skill_cutover (id INTEGER PRIMARY KEY, status TEXT NOT NULL, migrated_skills INTEGER NOT NULL DEFAULT 0, migrated_activations INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '', completed_at TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO extension_agent_skill_cutover(id,status) VALUES(1,'pending')`,
		`CREATE TABLE extension_agent_skill_metadata (id TEXT PRIMARY KEY, extension_id TEXT NOT NULL, space_id TEXT NOT NULL, name TEXT NOT NULL, scope_type TEXT NOT NULL, scope_id TEXT NOT NULL DEFAULT '', artifact_id TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 0, removed_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE extension_artifacts (artifact_id TEXT PRIMARY KEY, content_blob BLOB NOT NULL, archived_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE extension_agent_skill_activations (id TEXT PRIMARY KEY, activation_id TEXT NOT NULL, extension_id TEXT NOT NULL, space_id TEXT NOT NULL, character_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT '', trigger_type TEXT NOT NULL, explicit INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, loaded_tokens INTEGER NOT NULL DEFAULT 0, resource_reads INTEGER NOT NULL DEFAULT 0, resource_paths_json TEXT NOT NULL DEFAULT '[]', trace_id TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, agent_skill_name TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', scope_type TEXT NOT NULL DEFAULT '', compatibility_status TEXT NOT NULL DEFAULT '', instruction_position TEXT NOT NULL DEFAULT '', token_limit_hit INTEGER NOT NULL DEFAULT 0)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	artifact := legacySkillZip(t, map[string][]byte{
		"SKILL.md": []byte("---\nname: legacy-demo\ndescription: Use for legacy migration verification\nmetadata:\n  version: 1.0.0\n---\n\nUse the migrated instructions.\n"),
	})
	if err := db.Exec(`INSERT INTO extension_artifacts(artifact_id,content_blob) VALUES('artifact-legacy',?)`, artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO extension_agent_skill_metadata(id,extension_id,space_id,name,scope_type,scope_id,artifact_id,enabled) VALUES('legacy','local.agentskill.legacy-space.legacy-demo','legacy-space','legacy-demo','global','','artifact-legacy',1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO extension_agent_skill_activations(id,activation_id,extension_id,space_id,trigger_type,explicit,status,created_at,agent_skill_name) VALUES('activation-row','activation-legacy','local.agentskill.legacy-space.legacy-demo','legacy-space','explicit',1,'succeeded','2026-10-20T00:00:00Z','legacy-demo')`).Error; err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	container, err := kernel.NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()
	runtime, err := kernel.NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	if err := MigrateLegacyAgentSkills(ctx, db, runtime); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("extension_agent_skill_metadata") {
		t.Fatal("legacy metadata table was not removed")
	}
	items := container.AgentSkillCatalog.List(agent_skill.CatalogFilter{SpaceID: "legacy-space"})
	if len(items) != 1 || items[0].Name != "legacy-demo" || !items[0].Enabled {
		t.Fatalf("migrated skills = %#v", items)
	}
	var status string
	var migrated int
	if err := db.Raw(`SELECT status,migrated_skills FROM extension_agent_skill_cutover WHERE id=1`).Row().Scan(&status, &migrated); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || migrated != 1 {
		t.Fatalf("cutover status=%s migrated=%d", status, migrated)
	}
	var activationExtensionID string
	if err := container.Store.DB().QueryRowContext(ctx, `SELECT extension_id FROM kernel_observability_invocations WHERE invocation_id='activation-legacy'`).Scan(&activationExtensionID); err != nil {
		t.Fatal(err)
	}
	if activationExtensionID != items[0].ExtensionID {
		t.Fatalf("activation extension id=%s want %s", activationExtensionID, items[0].ExtensionID)
	}
}

func legacySkillZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
