package sqlite

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskDefinitionPinPersistsAndCannotBeReplacedByCASOrUpsert(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	run := &task_runtime.TaskRun{TaskRunID: "pin-run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", DefinitionFingerprint: "source-pin", Status: task_runtime.RunStatusQueued, Generation: 1, Revision: 1, CreatedAt: time.Now().UTC()}
	if err := repo.PutTaskRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	actual, err := repo.GetTaskRun(t.Context(), run.TaskRunID)
	if err != nil || actual.DefinitionFingerprint != run.DefinitionFingerprint {
		t.Fatalf("definition pin lost on reload: %+v %v", actual, err)
	}
	listed, err := repo.ListTaskRuns(t.Context(), task_runtime.ListTasksFilter{ExtensionID: "extension"})
	if err != nil || len(listed) != 1 || listed[0].DefinitionFingerprint != run.DefinitionFingerprint {
		t.Fatalf("definition pin lost on listing: %v", err)
	}
	next := task_runtime.CloneTaskRun(actual)
	next.Revision++
	next.DefinitionFingerprint = "replacement"
	if changed, err := repo.UpdateTaskRunCAS(t.Context(), next, actual.Status, actual.Generation, actual.Revision); err != nil || changed {
		t.Fatalf("CAS replaced immutable pin: %v %v", changed, err)
	}
	if err := repo.PutTaskRun(t.Context(), next); !task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskStaleWrite) {
		t.Fatalf("upsert replaced immutable pin: %v", err)
	}
	next.DefinitionFingerprint = actual.DefinitionFingerprint
	if changed, err := repo.UpdateTaskRunCAS(t.Context(), next, actual.Status, actual.Generation, actual.Revision); err != nil || !changed {
		t.Fatalf("valid task update rejected: %v %v", changed, err)
	}
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatalf("restart migration failed: %v", err)
	}
}
