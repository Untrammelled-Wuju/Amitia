package sqlite

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskCreateOnceCannotReplaceConcurrentOrCompletedExecution(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	start := make(chan struct{})
	winners := make(chan *task_runtime.TaskRun, 32)
	errors := make(chan error, 32)
	var workers sync.WaitGroup
	for i := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			run := &task_runtime.TaskRun{TaskRunID: "stable-request", TaskDefinitionID: "task", DefinitionFingerprint: "pin", Status: task_runtime.RunStatusQueued, Generation: 1, Revision: 1, CreatedAt: time.Now().UTC(), InputHash: fmt.Sprintf("input-%d", i)}
			created, err := NewTaskRepository(db).CreateTaskRun(t.Context(), run)
			if err != nil {
				errors <- err
			} else if created {
				winners <- run
			}
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if len(winners) != 1 {
		t.Fatalf("同一请求创建了多个执行: %d", len(winners))
	}
	winner := <-winners
	repo := NewTaskRepository(db)
	current, err := repo.GetTaskRun(t.Context(), winner.TaskRunID)
	if err != nil || current == nil || current.InputHash != winner.InputHash {
		t.Fatalf("并发请求修改了胜出任务输入: %+v %v", current, err)
	}
	current.Status, current.Revision = task_runtime.RunStatusSucceeded, 2
	if err := repo.PutTaskRun(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	if created, err := repo.CreateTaskRun(t.Context(), winner); err != nil || created {
		t.Fatalf("请求重试替换了已完成任务: %v %v", created, err)
	}
	stored, err := repo.GetTaskRun(t.Context(), winner.TaskRunID)
	if err != nil || stored.Status != task_runtime.RunStatusSucceeded || stored.Revision != 2 || stored.InputHash != winner.InputHash {
		t.Fatalf("已完成任务被重新创建: %+v %v", stored, err)
	}
	if _, err := repo.GetTaskRun(t.Context(), "missing"); !task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskNotFound) {
		t.Fatalf("任务不存在与存储失败没有区分: %v", err)
	}
}
