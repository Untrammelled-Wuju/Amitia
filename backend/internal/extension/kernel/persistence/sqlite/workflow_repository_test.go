package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func openWorkflowTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "workflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWorkflowDefinitionRepositoryCallableAndHash(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewWorkflowDefinitionRepository(db)
	def := workflow.WorkflowDefinition{ID: "wf", Name: "workflow", Enabled: true, CallableByAgent: false, Nodes: []workflow.WorkflowNode{{ID: "one", Type: "wasm", TargetID: "module"}}}
	if err := repo.Save(context.Background(), def); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Get(context.Background(), "wf")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CallableByAgent {
		t.Fatal("callableByAgent was read from enabled")
	}
	if loaded.DefinitionHash != workflow.ComputeDefinitionHash(def) {
		t.Fatalf("unexpected definition hash: %s", loaded.DefinitionHash)
	}
}

func TestWorkflowExecutionRepositoryPersistsContextStepsAndIdempotency(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewWorkflowExecutionRepository(db)
	now := time.Now().UTC()
	run := workflow.WorkflowRun{
		ExecutionID: "run-1",
		WorkflowID:  "wf",
		Status:      workflow.RunStatusRunning,
		Input:       json.RawMessage(`{"value":1}`),
		Context:     workflow.ExecutionContext{InvocationID: "run-1", ScheduleID: "schedule", TriggerID: "trigger", ExtensionID: "ext", ModuleID: "module", Generation: 4, IdempotencyKey: "trigger-key"},
		Attempt:     1,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	_, created, err := repo.Start(context.Background(), run)
	if err != nil || !created {
		t.Fatalf("start failed: created=%v err=%v", created, err)
	}
	duplicate := run
	duplicate.ExecutionID = "run-2"
	existing, created, err := repo.Start(context.Background(), duplicate)
	if err != nil || created || existing.ExecutionID != "run-1" {
		t.Fatalf("idempotency failed: existing=%+v created=%v err=%v", existing, created, err)
	}
	finished := now.Add(time.Second)
	step := workflow.StepRun{ExecutionID: "run-1", WorkflowID: "wf", NodeID: "one", Status: "succeeded", Output: json.RawMessage(`{"ok":true}`), Attempt: 1, StartedAt: now, FinishedAt: &finished}
	if err := repo.SaveStep(context.Background(), step); err != nil {
		t.Fatal(err)
	}
	run.Status = workflow.RunStatusSucceeded
	run.Output = step.Output
	run.Steps = []workflow.StepResult{{NodeID: "one", Status: "succeeded", Output: step.Output, Attempt: 1}}
	run.FinishedAt = &finished
	run.UpdatedAt = finished
	if err := repo.Finish(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Get(context.Background(), "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != workflow.RunStatusSucceeded || loaded.Context.TriggerID != "trigger" || len(loaded.Steps) != 1 {
		t.Fatalf("unexpected workflow run: %+v", loaded)
	}
}

func TestWorkflowTriggerBindingPersistence(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewWorkflowDefinitionRepository(db)
	binding := workflow.TriggerBinding{BindingID: "binding", Type: workflow.TriggerTypeEvent, EventType: "message.created", WorkflowID: "wf", Input: json.RawMessage(`{}`), Generation: 7, Enabled: true}
	if err := repo.SaveTrigger(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	bindings, err := repo.ListTriggers(context.Background(), workflow.TriggerTypeEvent, "message.created", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].Generation != 7 {
		t.Fatalf("unexpected trigger bindings: %+v", bindings)
	}
}

func TestWorkflowLocalRevisionAndTemplatePersistence(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewWorkflowDefinitionRepository(db)
	ctx := context.Background()
	def := workflow.WorkflowDefinition{
		SchemaVersion: workflow.UserWorkflowSchemaVersion,
		ID:            "wf-local",
		Name:          "Local workflow",
		Description:   "first",
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{"type":"object"}`),
		Nodes:         []workflow.WorkflowNode{{ID: "one", Type: "wait", Step: workflow.WorkflowStepInput{Input: json.RawMessage(`{}`)}}},
		Enabled:       true,
		Source:        "user",
	}
	firstRevision, err := repo.SaveRevision(ctx, "user-a", def, "initial")
	if err != nil {
		t.Fatal(err)
	}
	duplicateRevision, err := repo.SaveRevision(ctx, "user-a", def, "duplicate state")
	if err != nil {
		t.Fatal(err)
	}
	if duplicateRevision.RevisionID != firstRevision.RevisionID {
		t.Fatalf("identical consecutive revision should be deduplicated: first=%s duplicate=%s", firstRevision.RevisionID, duplicateRevision.RevisionID)
	}
	def.Description = "second"
	if _, err := repo.SaveRevision(ctx, "user-a", def, "changed"); err != nil {
		t.Fatal(err)
	}
	revisions, err := repo.ListRevisions(ctx, "user-a", def.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0].RevisionNo != 2 || revisions[1].RevisionNo != 1 {
		t.Fatalf("unexpected revisions: %+v", revisions)
	}
	loadedRevision, err := repo.GetRevision(ctx, "user-a", def.ID, revisions[1].RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedRevision.Definition.Description != "first" {
		t.Fatalf("unexpected revision definition: %+v", loadedRevision.Definition)
	}
	if _, err := repo.GetRevision(ctx, "user-b", def.ID, revisions[0].RevisionID); err == nil {
		t.Fatal("revision should be isolated by owner")
	}

	template, err := repo.SaveTemplate(ctx, "user-a", "Reusable", "local only", def)
	if err != nil {
		t.Fatal(err)
	}
	templates, err := repo.ListTemplates(ctx, "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 || templates[0].TemplateID != template.TemplateID || templates[0].NodeCount != 1 {
		t.Fatalf("unexpected templates: %+v", templates)
	}
	if other, err := repo.ListTemplates(ctx, "user-b"); err != nil || len(other) != 0 {
		t.Fatalf("template owner isolation failed: items=%+v err=%v", other, err)
	}
	loadedTemplate, err := repo.GetTemplate(ctx, "user-a", template.TemplateID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedTemplate.Definition.Description != "second" {
		t.Fatalf("unexpected template definition: %+v", loadedTemplate.Definition)
	}
	if err := repo.DeleteTemplate(ctx, "user-a", template.TemplateID); err != nil {
		t.Fatal(err)
	}
	if templates, err := repo.ListTemplates(ctx, "user-a"); err != nil || len(templates) != 0 {
		t.Fatalf("template delete failed: items=%+v err=%v", templates, err)
	}
}

func TestWorkflowDefinitionDeleteCleansHistoryAndRuns(t *testing.T) {
	db := openWorkflowTestDB(t)
	ctx := context.Background()
	defRepo := NewWorkflowDefinitionRepository(db)
	execRepo := NewWorkflowExecutionRepository(db)
	def := workflow.WorkflowDefinition{
		SchemaVersion: workflow.UserWorkflowSchemaVersion,
		ID:            "wf-delete-local",
		Name:          "Delete me",
		InputSchema:   json.RawMessage(`{"type":"object"}`),
		OutputSchema:  json.RawMessage(`{"type":"object"}`),
		Nodes:         []workflow.WorkflowNode{{ID: "one", Type: "wait", Step: workflow.WorkflowStepInput{Input: json.RawMessage(`{}`)}}},
		Enabled:       true,
		Source:        "user",
	}
	if err := defRepo.Save(ctx, def); err != nil {
		t.Fatal(err)
	}
	if _, err := defRepo.SaveRevision(ctx, "user-a", def, "snapshot"); err != nil {
		t.Fatal(err)
	}
	if err := defRepo.SaveTrigger(ctx, workflow.TriggerBinding{
		BindingID: "binding-delete", Type: workflow.TriggerTypeEvent, EventType: "user:user-a:test",
		WorkflowID: def.ID, Input: json.RawMessage(`{}`), Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, created, err := execRepo.Start(ctx, workflow.WorkflowRun{
		ExecutionID: "run-delete", WorkflowID: def.ID, Status: workflow.RunStatusRunning,
		Input: json.RawMessage(`{}`), Context: workflow.ExecutionContext{InvocationID: "run-delete"},
		Attempt: 1, StartedAt: now, UpdatedAt: now,
	}); err != nil || !created {
		t.Fatalf("create workflow run: created=%v err=%v", created, err)
	}
	if err := defRepo.Delete(ctx, def.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := defRepo.Get(ctx, def.ID); err == nil {
		t.Fatal("workflow definition should be deleted")
	}
	if revisions, err := defRepo.ListRevisions(ctx, "user-a", def.ID, 10); err != nil || len(revisions) != 0 {
		t.Fatalf("workflow revisions should be deleted: items=%+v err=%v", revisions, err)
	}
	if bindings, err := defRepo.ListTriggers(ctx, workflow.TriggerTypeEvent, "user:user-a:test", ""); err != nil || len(bindings) != 0 {
		t.Fatalf("workflow trigger bindings should be deleted: items=%+v err=%v", bindings, err)
	}
	if _, err := execRepo.Get(ctx, "run-delete"); err == nil {
		t.Fatal("workflow execution should be deleted")
	}
}

func TestWorkflowDefinitionDeletePreservesExtensionRunHistory(t *testing.T) {
	db := openWorkflowTestDB(t)
	ctx := context.Background()
	defRepo := NewWorkflowDefinitionRepository(db)
	execRepo := NewWorkflowExecutionRepository(db)
	def := workflow.WorkflowDefinition{
		ID: "wf-extension-delete", Name: "Extension workflow", Enabled: true, Source: "extension",
		Nodes: []workflow.WorkflowNode{{ID: "one", Type: "wait", Step: workflow.WorkflowStepInput{Input: json.RawMessage(`{}`)}}},
	}
	if err := defRepo.Save(ctx, def); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, created, err := execRepo.Start(ctx, workflow.WorkflowRun{
		ExecutionID: "run-extension-delete", WorkflowID: def.ID, Status: workflow.RunStatusRunning,
		Input: json.RawMessage(`{}`), Context: workflow.ExecutionContext{InvocationID: "run-extension-delete"},
		Attempt: 1, StartedAt: now, UpdatedAt: now,
	}); err != nil || !created {
		t.Fatalf("create extension workflow run: created=%v err=%v", created, err)
	}
	if err := defRepo.Delete(ctx, def.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := execRepo.Get(ctx, "run-extension-delete"); err != nil {
		t.Fatalf("extension workflow history should be preserved: %v", err)
	}
}
