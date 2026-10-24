package sqlite

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestRound98RemoteProgressAndCheckpointCommitRequireCurrentLease(t *testing.T) {
	for _, operation := range []string{"progress", "checkpoint"} {
		for _, condition := range []string{"active", "expired", "expired-after-read", "replaced-lease", "replaced-attempt", "replaced-generation", "replaced-revision", "cancelled"} {
			t.Run(operation+"/"+condition, func(t *testing.T) {
				db := openWorkflowTestDB(t)
				repo := NewTaskRepository(db)
				now := time.Now().UTC()
				expires := now.Add(time.Minute)
				run := &task_runtime.TaskRun{
					TaskRunID: "run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module",
					DefinitionFingerprint: "fingerprint", Status: task_runtime.RunStatusRunning,
					ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice,
					ExecutionAttemptID: "attempt", LeaseID: "lease", LeaseExpiresAt: &expires,
					Generation: 2, Revision: 3, CreatedAt: now,
				}
				if err := repo.PutTaskRun(t.Context(), run); err != nil {
					t.Fatal(err)
				}
				switch condition {
				case "expired":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET lease_expires_at = ? WHERE task_run_id = 'run'`, now.Add(-time.Second))
				case "expired-after-read":
					deadline := time.Now().UTC().Add(70 * time.Millisecond)
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET lease_expires_at = ? WHERE task_run_id = 'run'`, deadline)
					time.Sleep(120 * time.Millisecond)
				case "replaced-lease":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET lease_id = 'replacement' WHERE task_run_id = 'run'`)
				case "replaced-attempt":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET execution_attempt_id = 'replacement' WHERE task_run_id = 'run'`)
				case "replaced-generation":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET generation = 3 WHERE task_run_id = 'run'`)
				case "replaced-revision":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET revision = 4 WHERE task_run_id = 'run'`)
				case "cancelled":
					_, _ = db.ExecContext(t.Context(), `UPDATE extension_task_runs SET status = 'cancelled' WHERE task_run_id = 'run'`)
				}
				var written bool
				var err error
				if operation == "progress" {
					written, err = repo.PutRemoteProgressCAS(t.Context(), run, 1, []byte(`{"message":"updated"}`))
				} else {
					written, err = repo.PutRemoteCheckpointCAS(t.Context(), run, &task_runtime.TaskCheckpoint{
						CheckpointID: "checkpoint", TaskRunID: "run", Version: 1,
						Payload: json.RawMessage(`{"state":1}`), PayloadHash: "hash", CreatedAt: now,
					})
				}
				if err != nil || written != (condition == "active") {
					t.Fatalf("commit guard result: written=%v err=%v", written, err)
				}
				if operation == "progress" {
					stored, err := repo.GetProgress(t.Context(), "run")
					if err != nil || (stored != nil) != written {
						t.Fatalf("unexpected progress persistence: %+v %v", stored, err)
					}
				} else {
					stored, err := repo.GetLatestCheckpoint(t.Context(), "run")
					if err != nil || (stored != nil) != written {
						t.Fatalf("unexpected checkpoint persistence: %+v %v", stored, err)
					}
				}
			})
		}
	}
}

func TestRound98RemoteCheckpointPreventsDuplicateVersionAndProgressRollback(t *testing.T) {
	db := openWorkflowTestDB(t)
	repo := NewTaskRepository(db)
	now := time.Now().UTC()
	expires := now.Add(time.Minute)
	run := &task_runtime.TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExtensionID: "extension", ModuleID: "module", DefinitionFingerprint: "fingerprint", Status: task_runtime.RunStatusRunning, ExecutionPlacement: task_runtime.TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt", LeaseID: "lease", LeaseExpiresAt: &expires, Generation: 1, Revision: 1, CreatedAt: now}
	if err := repo.PutTaskRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	for i, seq := range []int64{2, 1, 2, 3} {
		ok, err := repo.PutRemoteProgressCAS(t.Context(), run, seq, []byte(`{"message":"ok"}`))
		if err != nil || ok != (i == 0 || i == 3) {
			t.Fatalf("unexpected progress update: seq=%d ok=%v err=%v", seq, ok, err)
		}
	}
	stored, err := repo.GetProgress(t.Context(), "run")
	if err != nil || stored == nil || stored.Sequence != 3 {
		t.Fatalf("progress sequence decreased: %+v %v", stored, err)
	}
	for i, id := range []string{"first", "different-id-same-version"} {
		ok, err := repo.PutRemoteCheckpointCAS(t.Context(), run, &task_runtime.TaskCheckpoint{CheckpointID: id, TaskRunID: "run", Version: 1, Payload: json.RawMessage(`{"state":1}`), CreatedAt: now})
		if err != nil || ok != (i == 0) {
			t.Fatalf("duplicate checkpoint version allowed: id=%s ok=%v err=%v", id, ok, err)
		}
	}
}
