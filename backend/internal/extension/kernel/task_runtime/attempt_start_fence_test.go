package task_runtime

import "testing"

func TestExecutionAttemptCannotReplaceActiveOrChangedQueuedRun(t *testing.T) {
	for _, state := range []string{"running", "starting", "paused", "recovery_required", "revision", "generation"} {
		t.Run(state, func(t *testing.T) {
			run := &TaskRun{TaskRunID: "run", Status: RunStatusQueued, Generation: 2, Revision: 4, ExecutionAttemptID: "new-attempt"}
			current := CloneTaskRun(run)
			current.ExecutionAttemptID = "existing-attempt"
			switch state {
			case "revision":
				current.Revision++
			case "generation":
				current.Generation++
			default:
				current.Status = TaskRunStatus(state)
			}
			store := &pauseStore{run: current}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			if err := service.persistExecutionAttempt(t.Context(), run, run.ExecutionAttemptID, ""); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
				t.Fatalf("过期调度覆盖了当前执行身份: %v", err)
			}
			if store.run.ExecutionAttemptID != "existing-attempt" || store.run.Revision != current.Revision {
				t.Fatalf("拒绝过期调度后执行记录仍被修改: %+v", store.run)
			}
		})
	}
}
