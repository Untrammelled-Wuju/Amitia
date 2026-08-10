package interaction

import (
	"context"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type goalReaderAdapter struct {
	registry *decision.GoalRegistry
}

func NewGoalReaderAdapter(registry *decision.GoalRegistry) GoalReconciliationReader {
	return &goalReaderAdapter{registry: registry}
}

func (a *goalReaderAdapter) GetGoal(ctx context.Context, goalID string) (decision.Goal, bool) {
	if a.registry == nil {
		return decision.Goal{}, false
	}
	return a.registry.Get(goalID)
}

func (a *goalReaderAdapter) ActiveForScope(ctx context.Context, userID, characterID, conversationID string) []decision.Goal {
	if a.registry == nil {
		return nil
	}
	return a.registry.ActiveForScope(userID, characterID, conversationID)
}

type taskReaderAdapter struct {
	service *task_runtime.TaskRuntimeService
}

func NewTaskReaderAdapter(service *task_runtime.TaskRuntimeService) TaskReconciliationReader {
	return &taskReaderAdapter{service: service}
}

func (a *taskReaderAdapter) GetTaskRun(ctx context.Context, taskRunID string) (AgentTaskRef, bool) {
	if a.service == nil {
		return AgentTaskRef{}, false
	}
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil || run == nil {
		return AgentTaskRef{}, false
	}
	return AgentTaskRef{
		TaskRunID:    run.TaskRunID,
		InvocationID: run.InvocationID,
		Status:       string(run.Status),
		Generation:   run.Generation,
		Completed:    run.Status.IsTerminal(),
	}, true
}

func (a *taskReaderAdapter) ListTaskRunsByInteraction(ctx context.Context, invocationID string) []AgentTaskRef {
	if a.service == nil || invocationID == "" {
		return nil
	}
	runs, err := a.service.ListTaskRuns(ctx, task_runtime.ListTasksFilter{})
	if err != nil {
		return nil
	}
	result := make([]AgentTaskRef, 0)
	for _, run := range runs {
		if run == nil || run.InvocationID != invocationID {
			continue
		}
		result = append(result, AgentTaskRef{
			TaskRunID:    run.TaskRunID,
			InvocationID: run.InvocationID,
			Status:       string(run.Status),
			Generation:   run.Generation,
			Completed:    run.Status.IsTerminal(),
		})
	}
	return result
}

type workflowReaderAdapter struct {
	executor *workflow.WorkflowExecutor
}

func NewWorkflowReaderAdapter(executor *workflow.WorkflowExecutor) WorkflowReconciliationReader {
	return &workflowReaderAdapter{executor: executor}
}

func (a *workflowReaderAdapter) GetWorkflowRun(ctx context.Context, executionID string) (AgentWorkflowRef, bool) {
	if a.executor == nil {
		return AgentWorkflowRef{}, false
	}
	store := a.executor.RunStore()
	if store == nil {
		return AgentWorkflowRef{}, false
	}
	run, err := store.Get(ctx, executionID)
	if err != nil || run == nil {
		return AgentWorkflowRef{}, false
	}
	return AgentWorkflowRef{
		ExecutionID: run.ExecutionID,
		WorkflowID:  run.WorkflowID,
		Status:      string(run.Status),
		Completed:   run.Status == workflow.RunStatusSucceeded || run.Status == workflow.RunStatusFailed || run.Status == workflow.RunStatusCancelled || run.Status == workflow.RunStatusCompensated,
		Attempts:    run.Attempt,
	}, true
}

func (a *workflowReaderAdapter) ListWorkflowRunsByInteraction(ctx context.Context, invocationID string) []AgentWorkflowRef {
	if a.executor == nil || invocationID == "" {
		return nil
	}
	store := a.executor.RunStore()
	if store == nil {
		return nil
	}
	runs, err := store.ListRecoverable(ctx, 512)
	if err != nil {
		return nil
	}
	result := make([]AgentWorkflowRef, 0)
	for i := range runs {
		run := runs[i]
		if run.Context.InvocationID != invocationID {
			continue
		}
		result = append(result, AgentWorkflowRef{
			ExecutionID: run.ExecutionID,
			WorkflowID:  run.WorkflowID,
			Status:      string(run.Status),
			Completed:   run.Status == workflow.RunStatusSucceeded || run.Status == workflow.RunStatusFailed || run.Status == workflow.RunStatusCancelled || run.Status == workflow.RunStatusCompensated,
			Attempts:    run.Attempt,
		})
	}
	return result
}

type invocationReaderAdapter struct {
	store observability.InvocationStore
}

func NewInvocationReaderAdapter(store observability.InvocationStore) InvocationReconciliationReader {
	return &invocationReaderAdapter{store: store}
}

func (a *invocationReaderAdapter) GetInvocation(ctx context.Context, invocationID string) (AgentInvocationRef, bool) {
	if a.store == nil {
		return AgentInvocationRef{}, false
	}
	inv, err := a.store.GetInvocation(ctx, invocationID)
	if err != nil || inv == nil {
		return AgentInvocationRef{}, false
	}
	return AgentInvocationRef{
		InvocationID: inv.InvocationID,
		CapabilityID: inv.CapabilityID,
		Status:       string(inv.Status),
		Completed:    isInvocationCompleted(inv.Status),
	}, true
}

func (a *invocationReaderAdapter) ListInvocationsByInteraction(ctx context.Context, invocationID string) []AgentInvocationRef {
	if a.store == nil || invocationID == "" {
		return nil
	}
	records, _, err := a.store.ListInvocations(ctx, observability.InvocationFilter{ListOptions: observability.ListOptions{Limit: 1024}})
	if err != nil {
		return nil
	}
	result := make([]AgentInvocationRef, 0, len(records))
	for _, inv := range records {
		if inv.InvocationID != invocationID && inv.TraceID != invocationID && inv.OperationID != invocationID {
			continue
		}
		result = append(result, AgentInvocationRef{
			InvocationID: inv.InvocationID,
			CapabilityID: inv.CapabilityID,
			Status:       string(inv.Status),
			Completed:    isInvocationCompleted(inv.Status),
		})
	}
	return result
}

type noopAgentObservationReader struct{}

func NewNoopAgentObservationReader() AgentObservationReader {
	return noopAgentObservationReader{}
}

func (noopAgentObservationReader) ListObservationsByInteraction(ctx context.Context, interactionID string) []decision.Observation {
	return nil
}

type noopInvocationReader struct{}

func NewNoopInvocationReader() InvocationReconciliationReader {
	return noopInvocationReader{}
}

func (noopInvocationReader) GetInvocation(ctx context.Context, invocationID string) (AgentInvocationRef, bool) {
	return AgentInvocationRef{}, false
}

func (noopInvocationReader) ListInvocationsByInteraction(ctx context.Context, invocationID string) []AgentInvocationRef {
	return nil
}

func isInvocationCompleted(s observability.ExecutionStatus) bool {
	switch s {
	case observability.StatusSucceeded, observability.StatusFailed, observability.StatusCancelled, observability.StatusTimedOut, observability.StatusDenied:
		return true
	}
	return false
}
