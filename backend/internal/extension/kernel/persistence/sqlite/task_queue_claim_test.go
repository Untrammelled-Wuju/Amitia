package sqlite

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskQueueAtomicClaimAcrossConcurrentSchedulers(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	repo := NewTaskRepository(db)
	now := time.Now().UTC()
	if err := repo.EnqueueTask(t.Context(), &task_runtime.TaskQueueEntry{TaskRunID: "only-task", AvailableAt: now, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	claims := make(chan *task_runtime.TaskQueueEntry, 32)
	errors := make(chan error, 32)
	var workers sync.WaitGroup
	for i := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			entry, err := NewTaskRepository(db).DequeueTask(t.Context(), fmt.Sprintf("scheduler-%d", i), time.Minute)
			if err != nil {
				errors <- err
			} else if entry != nil {
				claims <- entry
			}
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if len(claims) != 1 {
		t.Fatalf("一个任务被多个调度者领取: %d", len(claims))
	}
	claim := <-claims
	stored, err := repo.GetQueueEntry(t.Context(), claim.TaskRunID)
	if err != nil || stored == nil || stored.LeaseOwner != claim.LeaseOwner || stored.LeaseExpiresAt == nil || !stored.LeaseExpiresAt.Equal(*claim.LeaseExpiresAt) {
		t.Fatalf("领取回执与持久化租约不一致: %+v %v", stored, err)
	}
}

func TestTaskQueueAtomicClaimPreservesPriorityAvailabilityAndLease(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	repo := NewTaskRepository(db)
	now := time.Now().UTC()
	for _, entry := range []*task_runtime.TaskQueueEntry{
		{TaskRunID: "low", Priority: 1, AvailableAt: now, CreatedAt: now},
		{TaskRunID: "high", Priority: 9, AvailableAt: now, CreatedAt: now},
		{TaskRunID: "future", Priority: 99, AvailableAt: now.Add(time.Hour), CreatedAt: now},
	} {
		if err := repo.EnqueueTask(t.Context(), entry); err != nil {
			t.Fatal(err)
		}
	}
	for _, expected := range []string{"high", "low", ""} {
		entry, err := repo.DequeueTask(t.Context(), "scheduler", time.Minute)
		if err != nil || expected == "" && entry != nil || expected != "" && (entry == nil || entry.TaskRunID != expected) {
			t.Fatalf("队列领取违反优先级、可用时间或租约: expected=%s actual=%+v err=%v", expected, entry, err)
		}
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE extension_task_queue SET lease_expires_at = ? WHERE task_run_id = ?`, now.Add(-time.Minute), "high"); err != nil {
		t.Fatal(err)
	}
	entry, err := repo.DequeueTask(t.Context(), "replacement", time.Minute)
	if err != nil || entry == nil || entry.TaskRunID != "high" || entry.LeaseOwner != "replacement" {
		t.Fatalf("到期队列租约未被原子重新领取: %+v %v", entry, err)
	}
}
