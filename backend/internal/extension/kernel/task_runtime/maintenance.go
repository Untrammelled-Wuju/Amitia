package task_runtime

import (
	"context"
	"errors"
	"time"
)

func ValidateMaxConcurrent(value int) error {
	if value < 1 || value > 20 {
		return errors.New("最大并发任务数须为 1 至 20")
	}
	return nil
}

func (s *TaskRuntimeService) MaxConcurrent() int {
	return int(s.limiter.globalMax.Load())
}

func (s *TaskRuntimeService) ApplyMaxConcurrent(value int) {
	s.limiter.globalMax.Store(int64(value))
}

type MaintenanceTask struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Status    TaskRunStatus          `json:"status"`
	UpdatedAt time.Time              `json:"updatedAt"`
	Placement TaskExecutionPlacement `json:"placement"`
}

type MaintenanceStatus struct {
	Source            string            `json:"source"`
	Running           bool              `json:"running"`
	ActiveCount       int               `json:"activeCount"`
	LocalProcessCount int               `json:"localProcessCount"`
	Truncated         bool              `json:"truncated"`
	Tasks             []MaintenanceTask `json:"tasks"`
}

func (s *TaskRuntimeService) MaintenanceStatus(ctx context.Context) (MaintenanceStatus, error) {
	out := MaintenanceStatus{Source: "task-runtime", Tasks: []MaintenanceTask{}}
	var err error
	out.ActiveCount, err = s.store.CountActive(ctx)
	if err != nil {
		return out, err
	}
	runs, err := s.store.ListTaskRuns(ctx, ListTasksFilter{Limit: 51})
	if err != nil {
		return out, err
	}
	s.mu.RLock()
	out.LocalProcessCount = s.processBudget.total
	s.mu.RUnlock()
	out.Running = out.ActiveCount > 0 || out.LocalProcessCount > 0
	out.Truncated = len(runs) > 50
	if out.Truncated {
		runs = runs[:50]
	}
	for _, run := range runs {
		updated := run.CreatedAt
		for _, value := range []*time.Time{run.QueuedAt, run.StartedAt, run.FinishedAt, run.PausedAt, run.ResumedAt, run.CancelRequestedAt, run.LastHeartbeatAt} {
			if value != nil && value.After(updated) {
				updated = *value
			}
		}
		out.Tasks = append(out.Tasks, MaintenanceTask{ID: run.TaskRunID, Title: run.TaskDefinitionID, Status: run.Status, UpdatedAt: updated, Placement: run.EffectiveExecutionPlacement()})
	}
	return out, nil
}
