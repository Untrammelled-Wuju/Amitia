package task_runtime

import (
	"context"
	"time"
)

func (s *TaskRuntimeService) cancelConfirmedPausedRemote(ctx context.Context, current *TaskRun, reason string, now time.Time) error {
	if err := s.confirmRemotePaused(ctx, current); err != nil {
		return err
	}
	cancelledRun := cloneTaskRun(current)
	cancelledRun.Status = RunStatusCancelled
	cancelledRun.CancelRequestedAt = &now
	cancelledRun.FinishedAt = &now
	cancelledRun.Revision = NextRevision(current.Revision)
	return s.mutateTaskRun(ctx, taskMutationParams{
		next:       cancelledRun,
		expected:   current.Status,
		generation: current.Generation,
		revision:   current.Revision,
		removeQ:    true,
		eventType:  TaskEventCancelled,
		eventMsg:   reason,
	})
}
