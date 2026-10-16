package schedule

import "context"

type HostWorkloadHandler func(context.Context, *ScheduleContributionDefinition, *ScheduleTriggerRecord) (*TargetExecutionResult, error)

type HostWorkloadAdapter struct{ handler HostWorkloadHandler }

func NewHostWorkloadAdapter(handler HostWorkloadHandler) *HostWorkloadAdapter {
	return &HostWorkloadAdapter{handler: handler}
}

func (a *HostWorkloadAdapter) Type() TargetType { return TargetTypeHostWorkload }

func (a *HostWorkloadAdapter) Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error) {
	if a.handler == nil {
		return &TargetExecutionResult{ErrorCode: ErrCodeTargetNotFound, ErrorMessage: "host workload handler unavailable"}, nil
	}
	return a.handler(ctx, def, trigger)
}
