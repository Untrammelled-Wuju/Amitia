package task_runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type checkpointFenceStore struct {
	completionFenceStore
	checkpoint     *TaskCheckpoint
	progressWrites int
	beforeTx       func()
}

func (s *checkpointFenceStore) GetLatestCheckpoint(context.Context, string) (*TaskCheckpoint, error) {
	return s.checkpoint, nil
}

func (s *checkpointFenceStore) PutCheckpoint(_ context.Context, cp *TaskCheckpoint) error {
	s.checkpoint = cp
	return nil
}

func (s *checkpointFenceStore) PutProgress(context.Context, string, int64, []byte) error {
	s.progressWrites++
	return nil
}

func (s *checkpointFenceStore) WithinTaskTx(ctx context.Context, fn func(context.Context) error) error {
	if s.beforeTx != nil {
		s.beforeTx()
	}
	return fn(ctx)
}

func TestTaskCheckpointFencesCancelledReplacedAndConcurrentUpdates(t *testing.T) {
	for _, scenario := range []string{"cancelled", "replaced", "cas", "stale_version", "bad_hash", "valid", "pausing"} {
		t.Run(scenario, func(t *testing.T) {
			svc, _, _, run, def := taskAuthorityFixture(t)
			run.Status = RunStatusRunning
			store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
			svc.store = store
			payload := json.RawMessage(`{"cursor":2,"private":"checkpoint"}`)
			hash := hashBytes(payload)
			switch scenario {
			case "cancelled":
				store.run.Status = RunStatusCancelling
			case "replaced":
				store.run.Generation++
			case "cas":
				store.reject = true
			case "stale_version":
				store.checkpoint = &TaskCheckpoint{Version: 2}
			case "bad_hash":
				hash = "incorrect"
			case "pausing":
				store.run.Status = RunStatusPausing
			}
			svc.handleCheckpoint(t.Context(), run, def, payload, hash, 2)
			if scenario == "valid" || scenario == "pausing" {
				if store.checkpoint == nil || store.checkpoint.Version != 2 || store.run.CheckpointID == nil || run.CheckpointID == nil || store.checkpoint.PayloadHash != hash {
					t.Fatal("confirmed checkpoint was not committed")
				}
			} else if run.CheckpointID != nil || store.run.CheckpointID != nil || scenario != "stale_version" && store.checkpoint != nil {
				t.Fatal("stale checkpoint changed stored execution")
			}
		})
	}
}

func TestTaskProgressFencesCancellationGenerationAndMissingOwnedAttempt(t *testing.T) {
	for _, scenario := range []string{"valid", "cancelled", "replaced", "concurrent_cancel", "owned_missing_attempt", "invalid_sequence"} {
		t.Run(scenario, func(t *testing.T) {
			svc, _, authority, run, _ := taskAuthorityFixture(t)
			run.Status = RunStatusRunning
			store := &checkpointFenceStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}
			svc.store = store
			ctx := t.Context()
			seq := int64(1)
			switch scenario {
			case "cancelled":
				store.run.Status = RunStatusCancelling
			case "replaced":
				store.run.Generation++
			case "concurrent_cancel":
				store.beforeTx = func() { store.run.Status = RunStatusCancelling }
			case "owned_missing_attempt":
				ctx = coordination.WithScope(ctx, authority)
			case "invalid_sequence":
				seq = 0
			}
			if scenario == "owned_missing_attempt" {
				svc.handleProgress(ctx, run.TaskRunID, seq, nil, nil, nil, "stage", "private progress")
			} else {
				svc.handleProgress(ctx, run.TaskRunID, seq, nil, nil, nil, "stage", "private progress", run)
			}
			want := 0
			if scenario == "valid" {
				want = 1
			}
			if store.progressWrites != want {
				t.Fatalf("progress writes: got %d, want %d", store.progressWrites, want)
			}
		})
	}
}
