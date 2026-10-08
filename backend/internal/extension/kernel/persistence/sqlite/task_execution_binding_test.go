package sqlite

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskExecutionBindingAndLeaseSurviveCASReloadAndRestart(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	created := time.Now().UTC().Truncate(time.Millisecond)
	run := &task_runtime.TaskRun{TaskRunID: "binding-run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", DefinitionFingerprint: "pin", Status: task_runtime.RunStatusQueued, Generation: 1, Revision: 1, CreatedAt: created}
	if err := repo.PutTaskRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	next := task_runtime.CloneTaskRun(run)
	next.Status, next.Revision = task_runtime.RunStatusRunning, 2
	next.ExecutionPlacement = task_runtime.TaskExecutionPlacementDevice
	next.ExecutionAttemptID = "attempt"
	next.ExecutionTarget = task_runtime.TaskExecutionTarget{SourceTaskDefinitionID: "original-device-task", SpaceID: "core", DeviceID: "device", RuntimeID: "runtime", RuntimeSessionID: "session", ConnectionGeneration: 9}
	next.ExecutionResolvedAt, next.ExecutionResolvedBy = &created, "core-router"
	expires := created.Add(time.Minute)
	next.LeaseID, next.LeaseExpiresAt, next.LastHeartbeatAt = "lease", &expires, &created
	if changed, err := repo.UpdateTaskRunCAS(t.Context(), next, run.Status, run.Generation, run.Revision); err != nil || !changed {
		t.Fatalf("binding CAS failed: %v %v", changed, err)
	}
	check := func(actual *task_runtime.TaskRun) {
		t.Helper()
		if actual == nil || actual.ExecutionPlacement != next.ExecutionPlacement || actual.ExecutionTarget != next.ExecutionTarget || actual.ExecutionAttemptID != next.ExecutionAttemptID || actual.ExecutionResolvedBy != next.ExecutionResolvedBy || actual.ExecutionResolvedAt == nil || !actual.ExecutionResolvedAt.Equal(created) || actual.LeaseID != next.LeaseID || actual.LeaseExpiresAt == nil || !actual.LeaseExpiresAt.Equal(expires) || actual.LastHeartbeatAt == nil || !actual.LastHeartbeatAt.Equal(created) {
			t.Fatalf("execution binding was lost: %+v", actual)
		}
	}
	actual, err := repo.GetTaskRun(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatal(err)
	}
	check(actual)
	listed, err := repo.ListTaskRuns(t.Context(), task_runtime.ListTasksFilter{ExtensionID: "extension"})
	if err != nil || len(listed) != 1 {
		t.Fatalf("listing failed: %v", err)
	}
	check(listed[0])
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatalf("restart migration failed: %v", err)
	}
	next.Revision++
	next.Status, next.PausedAt, next.PauseRequestedAt = task_runtime.RunStatusPaused, &created, &created
	checkpointID := "owner-confirmed-checkpoint"
	next.CheckpointID = &checkpointID
	if err := repo.PutTaskRun(t.Context(), next); err != nil {
		t.Fatal(err)
	}
	actual, err = NewTaskRepository(db).GetTaskRun(t.Context(), run.TaskRunID)
	if err != nil {
		t.Fatal(err)
	}
	check(actual)
	if actual.Status != task_runtime.RunStatusPaused || actual.PausedAt == nil || !actual.PausedAt.Equal(created) || actual.PauseRequestedAt == nil || !actual.PauseRequestedAt.Equal(created) || actual.CheckpointID == nil || *actual.CheckpointID != checkpointID {
		t.Fatalf("restart lost the confirmed stop receipt: %+v", actual)
	}
	stale := task_runtime.CloneTaskRun(next)
	stale.Revision++
	stale.LeaseID = "forged"
	if changed, err := repo.UpdateTaskRunCAS(t.Context(), stale, run.Status, run.Generation, run.Revision); err != nil || changed {
		t.Fatalf("stale execution replaced current lease: %v %v", changed, err)
	}
}
