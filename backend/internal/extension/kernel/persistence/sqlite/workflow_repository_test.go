package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

func openWorkflowTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "workflow.db"))
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
