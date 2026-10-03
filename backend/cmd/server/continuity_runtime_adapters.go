package main

import (
	"context"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/interaction"
)

type continuityWakeDispatcher struct {
	entry *interaction.UnifiedEntry
}

func (d *continuityWakeDispatcher) DispatchContinuityWake(ctx context.Context, wake continuity.WakeRequest) error {
	if d == nil || d.entry == nil {
		return nil
	}
	_, err := d.entry.Handle(ctx, &interaction.UnifiedEntryRequest{
		Channel:                  wake.Channel,
		Message:                  wake.Message,
		SpaceID:                  wake.SpaceID,
		CharacterID:              wake.CharacterID,
		ConversationID:           wake.ConversationID,
		PeerID:                   wake.PeerID,
		ThreadID:                 wake.ThreadID,
		RequestID:                wake.RequestID,
		Source:                   "proactive",
		ProactiveTaskInstruction: wake.Message,
		IsInternal:               true,
	})
	return err
}

type continuityTaskEventSink struct {
	bridge *continuity.RuntimeBridge
}

func (s *continuityTaskEventSink) TaskEvent(ctx context.Context, event task_runtime.TaskDomainEvent) error {
	if s == nil || s.bridge == nil {
		return nil
	}
	s.bridge.OnTask(ctx, continuity.TaskLifecycleSignal{
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
	return nil
}

func wireContinuityKernelObservers(container *kernel.Container, bridge *continuity.RuntimeBridge) {
	if container == nil || bridge == nil {
		return
	}
	if container.WorkflowExecutor != nil {
		container.WorkflowExecutor.AddRunLifecycleSink(func(ctx context.Context, event workflow.WorkflowRunLifecycleEvent) {
			bridge.OnWorkflow(ctx, continuity.WorkflowLifecycleSignal{
				Type: event.Type, WorkflowID: event.WorkflowID, ExecutionID: event.ExecutionID, InstallationID: event.InstallationID,
				SpaceID: event.SpaceID, CharacterID: event.CharacterID, ConversationID: event.ConversationID,
				OperationID: event.OperationID, InvocationID: event.InvocationID, DeviceID: event.DeviceID,
				Status: string(event.Status), Error: event.Error, Timestamp: event.Timestamp,
			})
		})
	}
	if container.TaskRuntimeService != nil {
		container.TaskRuntimeService.AddEventSink(&continuityTaskEventSink{bridge: bridge})
	}
	if container.ApprovalBroker != nil {
		container.ApprovalBroker.AddObservers(
			func(request execution.ApprovalRequest) error {
				bridge.OnApprovalRequested(context.Background(), approvalSignal(request))
				return nil
			},
			func(request execution.ApprovalRequest) error {
				bridge.OnApprovalResolved(context.Background(), approvalSignal(request))
				return nil
			},
		)
	}
}

func approvalSignal(request execution.ApprovalRequest) continuity.ApprovalLifecycleSignal {
	return continuity.ApprovalLifecycleSignal{
		Resolved:   request.Status != execution.ApprovalStatusPending,
		Approved:   request.Status == execution.ApprovalStatusApproved,
		ApprovalID: request.ID, SpaceID: request.SpaceID, ConversationID: request.ConversationID,
		RequestID: request.RequestID, ToolCallID: request.ToolCallID, Reason: string(request.Status),
	}
}
