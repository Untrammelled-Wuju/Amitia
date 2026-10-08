package sqlite

import (
	"fmt"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestDeviceTaskDefinitionCreateOncePreservesConcurrentWinner(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	start := make(chan struct{})
	winners := make(chan string, 32)
	failures := make(chan error, 32)
	var workers sync.WaitGroup
	for index := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			definition := &task_runtime.TaskDefinition{TaskID: "mesh-task-stable", ContributionID: "mesh-task-stable", ExtensionID: "extension", ModuleID: "module", Entry: fmt.Sprintf("entry-%d", index), RemoteSource: &task_runtime.DeviceTaskDefinitionSource{Reference: task_runtime.DeviceTaskDefinitionReference{CoreID: "core", DeviceID: "device", SourceTaskID: "task"}}}
			created, err := NewTaskRepository(db).CreateTaskDefinition(t.Context(), definition)
			if err != nil {
				failures <- err
			} else if created {
				winners <- definition.Entry
			}
		}()
	}
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if len(winners) != 1 {
		t.Fatalf("设备任务目录出现多个写入胜者: %d", len(winners))
	}
	winner := <-winners
	repository := NewTaskRepository(db)
	stored, err := repository.GetTaskDefinition(t.Context(), "mesh-task-stable")
	if err != nil || stored.Entry != winner {
		t.Fatalf("并发导入覆盖了来源任务: %+v %v", stored, err)
	}
	stored.Entry = "replacement"
	if created, err := repository.CreateTaskDefinition(t.Context(), stored); err != nil || created {
		t.Fatalf("目录重试覆盖了已有定义: %v %v", created, err)
	}
	unchanged, err := repository.GetTaskDefinition(t.Context(), stored.TaskID)
	if err != nil || unchanged.Entry != winner {
		t.Fatal("目录冲突改变了已有定义")
	}
	stored.RemoteSource = nil
	if _, err := repository.CreateTaskDefinition(t.Context(), stored); !task_runtime.IsTaskErrorCode(err, task_runtime.ErrTaskDefinitionInvalid) {
		t.Fatalf("本地任务误用目录导入入口: %v", err)
	}
}
