package task_runtime

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestLateRemoteClaimOutcomeCannotReactivateStoppedOrPausingTask(t *testing.T) {
	for _, status := range []TaskRunStatus{RunStatusRunning, RunStatusPausing, RunStatusPaused, RunStatusRecoveryRequired, RunStatusCancelled, RunStatusSucceeded} {
		t.Run(string(status), func(t *testing.T) {
			service, _, authority, run, definition := taskAuthorityFixture(t)
			run.TaskDefinitionID, run.Status, run.Generation, run.Revision, run.ExecutionAttemptID = definition.TaskID, status, 3, 5, "attempt"
			store := &ownedCallbackStore{ownedOutcomeStore: ownedOutcomeStore{completionFenceStore: completionFenceStore{run: CloneTaskRun(run)}}, definition: definition}
			service.store = store
			service.applyExecutionOutcome(coordination.WithScope(t.Context(), authority), run, definition, TaskExecutionOutcome{Status: RunStatusRunning, LeaseID: "late-lease"})
			if store.run.Status != status || store.run.Revision != 5 || run.Status != status || store.run.LeaseID == "late-lease" {
				t.Fatalf("迟到租约结果重新激活或覆盖任务状态: %+v", store.run)
			}
		})
	}
}
