package extension

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"gorm.io/gorm"
)

func TestLegacyWorkflowToCallableMigratesNodesAndLimits(t *testing.T) {
	old := legacyWorkflowDefinition{
		SchemaVersion: "workflow-v1",
		Steps: []legacyWorkflowStep{
			{ID: "first", Type: "transform", Input: json.RawMessage(`{"op":"pick","keys":["value"]}`), OnError: legacyWorkflowErrorPolicy{Mode: "fail"}},
			{ID: "second", Type: "transform", Input: json.RawMessage(`{"op":"set","path":"copied","value":"steps.first.value"}`), OnError: legacyWorkflowErrorPolicy{Mode: "continue", Default: json.RawMessage(`{"copied":false}`)}},
		},
		Output: json.RawMessage(`{"type":"object"}`),
		Limits: legacyWorkflowLimits{MaxSteps: 8, MaxExecutionDurationMS: 5000, MaxStepDurationMS: 1000},
	}
	got := legacyWorkflowToCallable(old, "dev.amitia.skill.example", "Example", "Migration", "system/amitia-core", "legacy-skill-migration")
	if got.ID != "dev.amitia.skill.example" || got.Name != "Example" || !got.CallableByAgent {
		t.Fatalf("unexpected migrated definition: %+v", got)
	}
	if len(got.Nodes) != 2 || got.Nodes[1].Step.OnError.Mode != "continue" || len(got.Nodes[1].Step.OnError.Default) == 0 {
		t.Fatalf("workflow nodes were not preserved: %+v", got.Nodes)
	}
	if got.Limits.MaxSteps != 8 || got.Limits.MaxExecutionDurationMS != 5000 {
		t.Fatalf("workflow limits were not preserved: %+v", got.Limits)
	}
}

func TestMigrateLegacyWorkflowSkillsPersistsKernelWorkflow(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "app.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE extensions (id TEXT PRIMARY KEY, extension_id TEXT NOT NULL UNIQUE, kind TEXT NOT NULL DEFAULT 'Skill', name TEXT NOT NULL DEFAULT '', current_version TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 0, manifest_json TEXT NOT NULL DEFAULT '{}', normalized_manifest_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '', archived_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE extension_workshop_sessions (id TEXT PRIMARY KEY, space_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE extension_artifacts (artifact_id TEXT PRIMARY KEY, extension_id TEXT NOT NULL, extension_version TEXT NOT NULL, source TEXT NOT NULL DEFAULT '', session_id TEXT NOT NULL DEFAULT '', manifest_json TEXT NOT NULL DEFAULT '{}', workflow_json TEXT NOT NULL DEFAULT '{}', archived_at TEXT NOT NULL DEFAULT '')`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"metadata":{"id":"dev.amitia.skill.example","name":"Example","description":"Migration","version":"1.0.0"}}`
	workflow := `{"schemaVersion":"workflow-v1","steps":[{"id":"first","type":"transform","input":{"op":"pick","keys":["value"]},"onError":{"mode":"fail"}}],"output":{"type":"object"},"limits":{"maxSteps":8,"maxExecutionDurationMs":5000,"maxStepDurationMs":1000}}`
	if err := db.Exec(`INSERT INTO extensions (id, extension_id, current_version, source, enabled) VALUES ('ext-1', 'dev.amitia.skill.example', '1.0.0', 'workshop', 0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO extension_workshop_sessions (id, space_id) VALUES ('session-1', 'user-1')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO extension_artifacts (artifact_id, extension_id, extension_version, source, session_id, manifest_json, workflow_json) VALUES ('artifact-1', 'dev.amitia.skill.example', '1.0.0', 'workshop', 'session-1', ?, ?)`, manifest, workflow).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	container, err := extensionkernel.NewContainerBuilder().WithDBPath(dbPath).WithExtensionRoot(filepath.Join(root, "extensions")).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	reopenedSQL, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedSQL.Close()
	report, err := MigrateLegacyWorkflowSkills(context.Background(), db, container)
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 || report.Failed != 0 {
		t.Fatalf("unexpected migration report: %+v", report)
	}
	definition, err := container.WorkflowDefRepo.Get(context.Background(), "dev.amitia.skill.example")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Name != "Example" || len(definition.Nodes) != 1 || definition.Metadata["ownerSpaceId"] != "user-1" {
		t.Fatalf("migrated workflow mismatch: %+v", definition)
	}
}
