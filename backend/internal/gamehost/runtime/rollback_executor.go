package runtime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RollbackExecutor interface {
	Execute(ctx context.Context, plan LifecyclePlan, handleStore *RuntimeHandleStore, serviceExecutor ServiceExecutor, progress *StartupProgress) *RollbackResult
}

type rollbackExecutor struct{}

func NewRollbackExecutor() RollbackExecutor {
	return &rollbackExecutor{}
}

func (e *rollbackExecutor) Execute(ctx context.Context, plan LifecyclePlan, handleStore *RuntimeHandleStore, serviceExecutor ServiceExecutor, progress *StartupProgress) *RollbackResult {
	result := NewRollbackResult(string(plan.RuntimeID))
	if plan.Action != LifecycleActionStop {
		return result
	}

	for _, stage := range plan.Stages {
		for _, entry := range stage.Services {
			if !progress.IsStarted(entry.ServiceID) {
				continue
			}

			handle, found := handleStore.Get(plan.RuntimeID, entry.ServiceID)
			if !found {
				continue
			}

			err := serviceExecutor.Stop(ctx, *handle, false)
			if err != nil {
				result.AddError(err)
			} else {
				result.AddStopped(string(entry.ServiceID))
				handleStore.Remove(plan.RuntimeID, entry.ServiceID)
			}
		}
	}

	return result
}

func (p *StartupProgress) RecordStarted(serviceID domain.ServiceID) {
	p.StartedThisOperation = append(p.StartedThisOperation, serviceID)
}

func (p *StartupProgress) IsStarted(serviceID domain.ServiceID) bool {
	for _, id := range p.StartedThisOperation {
		if id == serviceID {
			return true
		}
	}
	return false
}

func BuildRollbackPlanFromProgress(
	progress StartupProgress,
	topology RuntimeTopologySnapshot,
	graph DependencyGraphSnapshot,
) (LifecyclePlan, error) {
	return NewLifecyclePlanner().BuildRollbackPlan(progress, topology, graph)
}

func nextBackoffDuration(attempt int, baseDelay time.Duration) time.Duration {
	delay := baseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * 2.0)
		if delay > 30*time.Second {
			delay = 30 * time.Second
			break
		}
	}
	return delay
}
