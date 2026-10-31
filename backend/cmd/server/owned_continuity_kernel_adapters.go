package main

import (
	"context"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	amitialog "log"
)

type ownedContinuityTaskEventSink struct {
	runtime *ownedContinuityRuntime
}

func (s *ownedContinuityTaskEventSink) TaskEvent(ctx context.Context, event task_runtime.TaskDomainEvent) error {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.ApplyTask(ctx, continuity.TaskLifecycleSignal{
		Type:         string(event.Type),
		TaskRunID:    event.Run.TaskRunID,
		SpaceID:      event.Run.ExecutionTarget.SpaceID.String(),
		OperationID:  event.Run.OperationID,
		InvocationID: event.Run.InvocationID,
		DeviceID:     event.Run.ExecutionTarget.DeviceID.String(),
		Status:       string(event.Run.Status),
		Reason:       event.Reason,
		ErrorCode:    event.ErrorCode,
		Timestamp:    event.OccurredAt,
	})
}

func wireOwnedContinuityKernelObservers(container *kernel.Container, runtime *ownedContinuityRuntime) {
	if container == nil || runtime == nil {
		return
	}
	if container.WorkflowExecutor != nil {
		container.WorkflowExecutor.AddRunLifecycleSink(func(ctx context.Context, event workflow.WorkflowRunLifecycleEvent) {
			if err := runtime.ApplyWorkflow(ctx, continuity.WorkflowLifecycleSignal{
				Type: event.Type, WorkflowID: event.WorkflowID, ExecutionID: event.ExecutionID,
				InstallationID: event.InstallationID, SpaceID: event.SpaceID, CharacterID: event.CharacterID,
				ConversationID: event.ConversationID, OperationID: event.OperationID, InvocationID: event.InvocationID,
				DeviceID: event.DeviceID, Status: string(event.Status), Error: event.Error, Timestamp: event.Timestamp,
			}); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		})
	}
	if container.TaskRuntimeService != nil {
		container.TaskRuntimeService.AddEventSink(&ownedContinuityTaskEventSink{runtime: runtime})
	}
	if container.ApprovalBroker != nil {
		container.ApprovalBroker.AddObservers(
			func(request execution.ApprovalRequest) error {
				return runtime.ApplyApprovalRequested(context.Background(), ownedContinuityApprovalSignal(request))
			},
			func(request execution.ApprovalRequest) error {
				return runtime.ApplyApprovalResolved(context.Background(), ownedContinuityApprovalSignal(request))
			},
		)
	}
}

func ownedContinuityApprovalSignal(request execution.ApprovalRequest) continuity.ApprovalLifecycleSignal {
	return continuity.ApprovalLifecycleSignal{
		Resolved: request.Status != execution.ApprovalStatusPending, Approved: request.Status == execution.ApprovalStatusApproved,
		ApprovalID: request.ID, SpaceID: request.SpaceID, ConversationID: request.ConversationID,
		RequestID: request.RequestID, ToolCallID: request.ToolCallID, Reason: string(request.Status),
	}
}
