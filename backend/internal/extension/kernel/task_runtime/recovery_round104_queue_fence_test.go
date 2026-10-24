package task_runtime

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type round104RecoveryQueueStore struct {
	*pauseStore
	transactions int
	beforeQueue  func()
}

func (s *round104RecoveryQueueStore) WithinTaskTx(ctx context.Context, fn func(context.Context) error) error {
	s.transactions++
	if s.transactions == 2 && s.beforeQueue != nil {
		beforeQueue := s.beforeQueue
		s.beforeQueue = nil
		beforeQueue()
	}
	return s.pauseStore.WithinTaskTx(ctx, fn)
}

func TestRound104RecoverCannotQueueAfterOwnerAdvancesCheckpoint(t *testing.T) {
	svc, ownerCtx, data, _, original, def, first := round16PendingCheckpointFixture(t)
	svc.config.OwnedOutcomes = AcknowledgedTaskOutcomePort{Data: data}
	svc.config.OwnedExecutionGuard = func(ctx context.Context, scope coordination.ExecutionScope, _ *TaskRun) (context.Context, func(), error) {
		return coordination.WithScope(ctx, scope), func() {}, nil
	}
	store := &round104RecoveryQueueStore{pauseStore: &pauseStore{run: CloneTaskRun(original), def: def}}
	store.beforeQueue = func() {
		if store.run.CheckpointID == nil || *store.run.CheckpointID != first.CheckpointID {
			t.Fatal("Owner第一版检查点尚未投影")
		}
		staged := CloneTaskRun(original)
		checkpointID := first.CheckpointID
		staged.CheckpointID = &checkpointID
		payload := json.RawMessage(`{"cursor":2,"data":{"step":"other-core"}}`)
		second := &TaskCheckpoint{
			CheckpointID: ownedStableCheckpointID(staged, 2), TaskRunID: original.TaskRunID,
			Version: 2, Payload: payload, PayloadHash: hashBytes(payload),
			DefinitionHash: def.DefinitionHash, InputHash: original.InputHash,
			CreatedAt: time.Now().UTC(),
		}
		if err := (AcknowledgedTaskCheckpointPort{Data: data}).SaveCheckpoint(ownerCtx, staged, second); err != nil {
			t.Fatalf("提交另一 Core 检查点: %v", err)
		}
	}
	svc.store = store
	svc.queue = NewTaskQueue(store, "round104", time.Minute)
	atomic.StoreInt32(&svc.dispatching, 1)
	_, err := svc.Recover(t.Context(), original.TaskRunID)
	if !IsTaskErrorCode(err, ErrTaskCheckpointIncompatible) || store.transactions != 2 || store.queued != 0 || store.run.Status != RunStatusRecoveryRequired || store.run.Generation != original.Generation {
		t.Fatalf("新的Owner检查点已提交，仍可能按旧检查点排队: error=%v transactions=%d queued=%d status=%s generation=%d", err, store.transactions, store.queued, store.run.Status, store.run.Generation)
	}
}
