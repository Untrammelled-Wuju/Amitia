package continuity

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type WorkflowLifecycleSignal struct {
	Type           string
	WorkflowID     string
	ExecutionID    string
	InstallationID string
	SpaceID        string
	CharacterID    string
	ConversationID string
	OperationID    string
	InvocationID   string
	DeviceID       string
	Status         string
	Error          string
	Timestamp      time.Time
}

type TaskLifecycleSignal struct {
	Type         string
	TaskRunID    string
	SpaceID      string
	OperationID  string
	InvocationID string
	DeviceID     string
	Status       string
	Reason       string
	ErrorCode    string
	Timestamp    time.Time
}

type ApprovalLifecycleSignal struct {
	Resolved       bool
	Approved       bool
	ApprovalID     string
	SpaceID        string
	ConversationID string
	RequestID      string
	ToolCallID     string
	Reason         string
	Timestamp      time.Time
}

type RuntimeBridge struct {
	repo  *Repository
	waits *WaitCoordinator
}

func NewRuntimeBridge(repo *Repository, waits *WaitCoordinator) *RuntimeBridge {
	return &RuntimeBridge{repo: repo, waits: waits}
}

func (b *RuntimeBridge) OnWorkflow(ctx context.Context, signal WorkflowLifecycleSignal) {
	if b == nil || b.repo == nil {
		return
	}
	thread, _ := b.repo.FindThreadByBindings(signal.SpaceID,
		BindingLookup{Type: "workflow_run", ID: signal.ExecutionID},
		BindingLookup{Type: "execution", ID: signal.ExecutionID},
		BindingLookup{Type: "request", ID: signal.OperationID},
		BindingLookup{Type: "conversation", ID: signal.ConversationID},
	)
	if thread == nil {
		return
	}
	_ = b.repo.Bind(thread.ID, "workflow_run", signal.ExecutionID, "execution", "workflow_lifecycle", 1)
	_ = b.repo.Bind(thread.ID, "execution", signal.ExecutionID, "execution", "workflow_lifecycle", 1)
	b.appendRuntimeEvent(thread.ID, "workflow."+normalizeRuntimeEvent(signal.Type, signal.Status), signal.ExecutionID, signal.Timestamp, map[string]any{
		"workflowId": signal.WorkflowID, "executionId": signal.ExecutionID, "status": signal.Status, "deviceId": signal.DeviceID, "error": signal.Error,
	})

	status := strings.ToLower(strings.TrimSpace(signal.Status))
	if strings.Contains(status, "waiting_device") && signal.DeviceID != "" {
		b.ensureRuntimeWait(thread, WaitTypeDevice, "等待设备上线以继续工作流", map[string]any{"deviceId": signal.DeviceID, "executionId": signal.ExecutionID}, signal.ExecutionID, false)
	}
	if runtimeTerminalStatus(status) && b.waits != nil {
		_, _ = b.waits.Signal(ctx, Signal{WaitType: WaitTypeDependency, SpaceID: thread.SpaceID, Source: "workflow", SourceID: signal.ExecutionID, Attributes: map[string]any{"executionId": signal.ExecutionID, "workflowRunId": signal.ExecutionID, "operationId": signal.OperationID, "status": status}, OccurredAt: eventTime(signal.Timestamp)})
		if status == "succeeded" || status == "completed" {
			b.updateRuntimeSuccess(thread, "工作流执行已完成")
		} else {
			b.updateRuntimeFailure(thread, "工作流执行未成功："+firstNonEmpty(signal.Error, status))
		}
	}
}

func (b *RuntimeBridge) OnTask(ctx context.Context, signal TaskLifecycleSignal) {
	if b == nil || b.repo == nil {
		return
	}
	thread, _ := b.repo.FindThreadByBindings(signal.SpaceID,
		BindingLookup{Type: "task_run", ID: signal.TaskRunID},
		BindingLookup{Type: "request", ID: signal.OperationID},
		BindingLookup{Type: "execution", ID: signal.InvocationID},
	)
	if thread == nil {
		return
	}
	_ = b.repo.Bind(thread.ID, "task_run", signal.TaskRunID, "execution", "task_lifecycle", 1)
	b.appendRuntimeEvent(thread.ID, "task."+normalizeRuntimeEvent(signal.Type, signal.Status), signal.TaskRunID, signal.Timestamp, map[string]any{
		"taskRunId": signal.TaskRunID, "operationId": signal.OperationID, "invocationId": signal.InvocationID, "status": signal.Status, "deviceId": signal.DeviceID, "reason": signal.Reason, "errorCode": signal.ErrorCode,
	})
	status := strings.ToLower(strings.TrimSpace(signal.Status))
	if strings.Contains(status, "waiting_device") && signal.DeviceID != "" {
		b.ensureRuntimeWait(thread, WaitTypeDevice, "等待设备上线以继续任务", map[string]any{"deviceId": signal.DeviceID, "taskRunId": signal.TaskRunID}, signal.TaskRunID, false)
	}
	if runtimeTerminalStatus(status) && b.waits != nil {
		_, _ = b.waits.Signal(ctx, Signal{WaitType: WaitTypeDependency, SpaceID: thread.SpaceID, Source: "task_runtime", SourceID: signal.TaskRunID, Attributes: map[string]any{"taskRunId": signal.TaskRunID, "operationId": signal.OperationID, "invocationId": signal.InvocationID, "status": status}, OccurredAt: eventTime(signal.Timestamp)})
		if status == "succeeded" || status == "completed" {
			b.updateRuntimeSuccess(thread, "任务执行已完成")
		} else {
			b.updateRuntimeFailure(thread, "任务执行未成功："+firstNonEmpty(signal.Reason, signal.ErrorCode, status))
		}
	}
}

func (b *RuntimeBridge) OnApprovalRequested(_ context.Context, signal ApprovalLifecycleSignal) {
	if b == nil || b.repo == nil {
		return
	}
	thread, _ := b.repo.FindThreadByBindings(signal.SpaceID,
		BindingLookup{Type: "request", ID: signal.RequestID},
		BindingLookup{Type: "conversation", ID: signal.ConversationID},
	)
	if thread == nil {
		return
	}
	condition := map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID}
	b.ensureRuntimeWait(thread, WaitTypeApproval, "等待用户审批后继续", condition, signal.RequestID, false)
	b.appendRuntimeEvent(thread.ID, "approval.requested", signal.ApprovalID, signal.Timestamp, condition)
}

func (b *RuntimeBridge) OnApprovalResolved(ctx context.Context, signal ApprovalLifecycleSignal) {
	if b == nil || b.waits == nil {
		return
	}
	_, _ = b.waits.Signal(ctx, Signal{WaitType: WaitTypeApproval, SpaceID: signal.SpaceID, Source: "approval", SourceID: signal.ApprovalID, Attributes: map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID, "approved": signal.Approved}, OccurredAt: eventTime(signal.Timestamp)})
}

func (b *RuntimeBridge) OnDeviceReady(ctx context.Context, spaceID, deviceID string) {
	if b == nil || b.waits == nil || strings.TrimSpace(deviceID) == "" {
		return
	}
	_, _ = b.waits.Signal(ctx, Signal{WaitType: WaitTypeDevice, SpaceID: spaceID, Source: "device_ready", SourceID: deviceID, Attributes: map[string]any{"deviceId": deviceID}, OccurredAt: time.Now().UTC()})
}

func (b *RuntimeBridge) ensureRuntimeWait(thread *Thread, waitType, description string, condition map[string]any, sourceExecutionID string, autoResume bool) {
	if thread == nil {
		return
	}
	conditionJSON, _ := json.Marshal(condition)
	open, _ := b.repo.ListOpenWaits(thread.ID, 100)
	for i := range open {
		if open[i].WaitType == waitType && open[i].SourceExecutionID == sourceExecutionID && open[i].ConditionJSON == string(conditionJSON) {
			return
		}
	}
	_ = b.repo.CreateWait(&Wait{ThreadID: thread.ID, WaitType: waitType, Status: WaitStatusWaiting, Description: description, ConditionJSON: string(conditionJSON), ResumeHint: "条件满足后重新评估持续事项", SourceExecutionID: sourceExecutionID, AutoResume: autoResume})
	latest, _ := b.repo.GetThread(thread.ID, "")
	if latest != nil && latest.Status == ThreadStatusActive {
		_, _ = b.repo.UpdateThreadCAS(latest.ID, latest.Revision, map[string]interface{}{"status": ThreadStatusWaiting})
	}
}

func (b *RuntimeBridge) appendRuntimeEvent(threadID, eventType, sourceID string, at time.Time, payload map[string]any) {
	body, _ := json.Marshal(payload)
	if at.IsZero() {
		at = time.Now().UTC()
	}
	_, _ = b.repo.AppendEvent(&ThreadEvent{ThreadID: threadID, EventType: eventType, SourceType: "runtime", SourceID: sourceID, PayloadJSON: string(body), IdempotencyKey: eventType + "|" + sourceID + "|" + at.UTC().Format(time.RFC3339Nano), OccurredAt: at.UTC()})
}

func (b *RuntimeBridge) updateRuntimeSuccess(thread *Thread, state string) {
	latest, _ := b.repo.GetThread(thread.ID, "")
	if latest == nil || latest.Status.IsTerminal() || latest.Status == ThreadStatusPaused {
		return
	}
	open, _ := b.repo.ListOpenWaits(latest.ID, 1)
	updates := map[string]interface{}{"current_state": shortText(state, 500), "last_active_at": time.Now().UTC()}
	if len(open) == 0 && (latest.Status == ThreadStatusWaiting || latest.Status == ThreadStatusBlocked) {
		updates["status"] = ThreadStatusActive
	}
	_, _ = b.repo.UpdateThreadCAS(latest.ID, latest.Revision, updates)
}

func (b *RuntimeBridge) updateRuntimeFailure(thread *Thread, state string) {
	latest, _ := b.repo.GetThread(thread.ID, "")
	if latest == nil || latest.Status.IsTerminal() {
		return
	}
	_, _ = b.repo.UpdateThreadCAS(latest.ID, latest.Revision, map[string]interface{}{"status": ThreadStatusBlocked, "current_state": shortText(state, 500), "next_action": "评估失败原因并决定重试、修复或调整方案", "last_active_at": time.Now().UTC()})
}

func runtimeTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "completed", "failed", "cancelled", "canceled", "interrupted", "timed_out", "timeout",
		"cancel_timeout", "cancel_failed", "dropped", "compensated", "compensation_failed",
		"manual_intervention_required", "manual_intervention":
		return true
	default:
		return false
	}
}

func normalizeRuntimeEvent(kind, status string) string {
	value := strings.ToLower(strings.TrimSpace(kind))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(status))
	}
	value = strings.ReplaceAll(value, " ", "_")
	if value == "" {
		return "updated"
	}
	return value
}

func eventTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
