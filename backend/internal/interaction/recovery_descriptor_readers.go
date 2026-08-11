package interaction

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
)

type goalRecoveryReaderAdapter struct {
	registry *decision.GoalRegistry
}

func NewGoalRecoveryReaderAdapter(registry *decision.GoalRegistry) GoalRecoveryReader {
	return &goalRecoveryReaderAdapter{registry: registry}
}

func (a *goalRecoveryReaderAdapter) GetGoal(ctx context.Context, id string) (decision.Goal, bool) {
	if a.registry == nil {
		return decision.Goal{}, false
	}
	return a.registry.Get(id)
}

type taskRecoveryReaderAdapter struct {
	service *task_runtime.TaskRuntimeService
}

func NewTaskRecoveryReaderAdapter(service *task_runtime.TaskRuntimeService) TaskRecoveryReader {
	return &taskRecoveryReaderAdapter{service: service}
}

func (a *taskRecoveryReaderAdapter) GetTaskRun(ctx context.Context, taskRunID string) (*taskRunRecoveryView, error) {
	if a.service == nil {
		return nil, fmt.Errorf("task runtime reader not configured")
	}
	run, err := a.service.GetTaskRun(ctx, taskRunID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("task not found: %s", taskRunID)
	}
	view := &taskRunRecoveryView{
		TaskRunID:        run.TaskRunID,
		TaskDefinitionID: run.TaskDefinitionID,
		Generation:       run.Generation,
		Status:           string(run.Status),
		InputHash:        run.InputHash,
	}
	if run.CheckpointID != nil {
		view.CheckpointID = run.CheckpointID
	}
	return view, nil
}

func (a *taskRecoveryReaderAdapter) GetLatestCheckpoint(ctx context.Context, taskRunID string) (*taskCheckpointRecoveryView, error) {
	if a.service == nil {
		return nil, fmt.Errorf("task runtime reader not configured")
	}
	cp, err := a.service.GetLatestCheckpoint(ctx, taskRunID)
	if err != nil || cp == nil {
		return nil, fmt.Errorf("checkpoint not found: %s", taskRunID)
	}
	return &taskCheckpointRecoveryView{
		CheckpointID:   cp.CheckpointID,
		Version:        cp.Version,
		DefinitionHash: cp.DefinitionHash,
		InputHash:      cp.InputHash,
	}, nil
}

type workflowRecoveryReaderAdapter struct {
	executor *workflow.WorkflowExecutor
}

func NewWorkflowRecoveryReaderAdapter(executor *workflow.WorkflowExecutor) WorkflowRecoveryReader {
	return &workflowRecoveryReaderAdapter{executor: executor}
}

func (a *workflowRecoveryReaderAdapter) GetRun(ctx context.Context, executionID string) (*workflowRunRecoveryView, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("workflow executor reader not configured")
	}
	store := a.executor.RunStore()
	if store == nil {
		return nil, fmt.Errorf("workflow run store not configured")
	}
	run, err := store.Get(ctx, executionID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("workflow run not found: %s", executionID)
	}
	return &workflowRunRecoveryView{
		WorkflowID:  run.WorkflowID,
		ExecutionID: run.ExecutionID,
		Generation:  run.Context.Generation,
		Status:      string(run.Status),
	}, nil
}

func (a *workflowRecoveryReaderAdapter) ListCheckpoints(ctx context.Context, executionID string) ([]workflowCheckpointRecoveryView, error) {
	if a.executor == nil {
		return nil, fmt.Errorf("workflow executor reader not configured")
	}
	store := a.executor.CheckpointStore()
	if store == nil {
		return nil, fmt.Errorf("workflow checkpoint store not configured")
	}
	cps, err := store.List(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints: %w", err)
	}
	result := make([]workflowCheckpointRecoveryView, 0, len(cps))
	for _, cp := range cps {
		result = append(result, workflowCheckpointRecoveryView{NodeID: cp.NodeID})
	}
	return result, nil
}

type invocationRecoveryReaderAdapter struct {
	store observability.InvocationStore
}

func NewInvocationRecoveryReaderAdapter(store observability.InvocationStore) InvocationRecoveryReader {
	return &invocationRecoveryReaderAdapter{store: store}
}

func (a *invocationRecoveryReaderAdapter) GetInvocation(ctx context.Context, invocationID string) (*invocationRecoveryView, error) {
	if a.store == nil {
		return nil, fmt.Errorf("invocation reader not configured")
	}
	inv, err := a.store.GetInvocation(ctx, invocationID)
	if err != nil || inv == nil {
		return nil, fmt.Errorf("invocation not found: %s", invocationID)
	}
	return &invocationRecoveryView{
		InvocationID: inv.InvocationID,
		OperationID:  inv.OperationID,
		TraceID:      inv.TraceID,
		CapabilityID: inv.CapabilityID,
		Status:       string(inv.Status),
	}, nil
}

type pipelineRecoveryReaderAdapter struct {
	manager *pipelinecheckpoint.Manager
}

func NewPipelineRecoveryReaderAdapter(manager *pipelinecheckpoint.Manager) PipelineRecoveryReader {
	return &pipelineRecoveryReaderAdapter{manager: manager}
}

func (a *pipelineRecoveryReaderAdapter) Load(ctx context.Context, conversationID, pipelineType string) (*pipelineRecordView, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("pipeline reader not configured")
	}
	rec, err := a.manager.Load(conversationID, pipelineType)
	if err != nil || rec == nil {
		return nil, err
	}
	return &pipelineRecordView{
		ConversationID:      rec.ConversationID,
		PipelineType:        rec.PipelineType,
		CheckpointVersion:   rec.CheckpointVersion,
		LastMessageSequence: rec.LastMessageSequence,
	}, nil
}
