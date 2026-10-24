package sqlite

import (
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestRound105RetryCreateOnceAcrossRepositories(t *testing.T) {
	db := openWorkflowTestDB(t)
	db.SetMaxOpenConns(1)
	const workers = 32
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan bool, workers)
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			candidate := &task_runtime.TaskRun{TaskRunID: "tr-repeatable-retry", TaskDefinitionID: "task", DefinitionFingerprint: "pinned", Status: task_runtime.RunStatusQueued, Generation: 2, Revision: 1, Attempt: 2, MaxAttempts: 3, CreatedAt: time.Now().UTC()}
			ok, err := NewTaskRepository(db).CreateRetryTaskRun(t.Context(), candidate)
			results <- ok
			if err != nil {
				errors <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	created := 0
	for ok := range results {
		if ok {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("multiple cores created retry execution %d times", created)
	}
	actual, err := NewTaskRepository(db).GetTaskRun(t.Context(), "tr-repeatable-retry")
	if err != nil || actual.Attempt != 2 || actual.Generation != 2 {
		t.Fatalf("retry not persisted: %+v %v", actual, err)
	}
	actual.Status, actual.Revision = task_runtime.RunStatusSucceeded, 2
	if err := NewTaskRepository(db).PutTaskRun(t.Context(), actual); err != nil {
		t.Fatal(err)
	}
	if again, err := NewTaskRepository(db).CreateRetryTaskRun(t.Context(), &task_runtime.TaskRun{TaskRunID: actual.TaskRunID, TaskDefinitionID: "task", DefinitionFingerprint: "pinned", Status: task_runtime.RunStatusQueued, Generation: 2, Revision: 1, Attempt: 2, MaxAttempts: 3}); err != nil || again {
		t.Fatalf("retry overwrote finished execution: %v %v", again, err)
	}
}

func TestRound105RetryRejectsMalformedInitialTask(t *testing.T) {
	repo := NewTaskRepository(openWorkflowTestDB(t))
	for _, candidate := range []*task_runtime.TaskRun{
		nil,
		{TaskRunID: "a", Revision: 2, Generation: 2, Attempt: 2, Status: task_runtime.RunStatusQueued},
		{TaskRunID: "b", Revision: 1, Generation: 1, Attempt: 2, Status: task_runtime.RunStatusQueued},
		{TaskRunID: "c", Revision: 1, Generation: 2, Attempt: 1, Status: task_runtime.RunStatusQueued},
		{TaskRunID: "d", Revision: 1, Generation: 2, Attempt: 2, Status: task_runtime.RunStatusRunning},
	} {
		if ok, err := repo.CreateRetryTaskRun(t.Context(), candidate); err == nil || ok {
			t.Fatalf("invalid retry created: %+v", candidate)
		}
	}
}
