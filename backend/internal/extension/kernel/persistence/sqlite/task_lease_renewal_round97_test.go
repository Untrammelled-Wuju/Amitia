package sqlite

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestRound97RemoteLeaseCASChecksPersistedExpiryAndIdentityInSameUpdate(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	for _, tc := range []struct {
		name         string
		delay        time.Duration
		alterField   string
		shouldUpdate bool
	}{
		{name: "active", shouldUpdate: true},
		{name: "expires-after-read", delay: 120 * time.Millisecond},
		{name: "lease-replaced-without-revision-change", alterField: "lease_id"},
		{name: "attempt-replaced-without-revision-change", alterField: "execution_attempt_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			expires := now.Add(70 * time.Millisecond)
			if tc.name != "expires-after-read" {
				expires = now.Add(time.Minute)
			}
			run := &task_runtime.TaskRun{
				TaskRunID: tc.name, TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module",
				DefinitionFingerprint: "pin", Status: task_runtime.RunStatusRunning,
				Generation: 2, Revision: 3, CreatedAt: now, LeaseID: "lease", LeaseExpiresAt: &expires,
				ExecutionAttemptID: "attempt", ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice,
			}
			if err := repo.PutTaskRun(t.Context(), run); err != nil {
				t.Fatal(err)
			}
			next := task_runtime.CloneTaskRun(run)
			future := now.Add(2 * time.Minute)
			next.Revision++
			next.LeaseExpiresAt = &future
			if tc.alterField != "" {
				if _, err := db.ExecContext(t.Context(), "UPDATE extension_task_runs SET "+tc.alterField+" = 'taken-over' WHERE task_run_id = ?", run.TaskRunID); err != nil {
					t.Fatal(err)
				}
			}
			if tc.delay > 0 {
				time.Sleep(tc.delay)
			}
			changed, err := repo.RenewRemoteLeaseCAS(t.Context(), next, run.Status, run.Generation, run.Revision)
			if err != nil || changed != tc.shouldUpdate {
				t.Fatalf("unexpected persisted renewal result: changed=%v err=%v", changed, err)
			}
			stored, err := repo.GetTaskRun(t.Context(), run.TaskRunID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.shouldUpdate && (stored.Revision != 4 || stored.LeaseExpiresAt == nil || !stored.LeaseExpiresAt.Equal(future)) {
				t.Fatalf("active renewal not persisted: %+v", stored)
			}
			if !tc.shouldUpdate && stored.Revision != 3 {
				t.Fatalf("stale renewal mutated task: %+v", stored)
			}
		})
	}
}
