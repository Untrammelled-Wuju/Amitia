package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
)

type ownedContinuityRuntime struct {
	engine        *business.Engine
	intelligence  *continuity.Intelligence
	semantic      *continuity.EmbeddingSemanticMatcher
	disambiguator *continuity.LLMThreadDisambiguator
	spaceID       string
	deviceID      string
	coreID        string
}

func newOwnedContinuityRuntime(engine *business.Engine, chatSvc continuity.WorkshopJSONGenerator, embeddingSvc continuity.Embedder, spaceID, deviceID, coreID string) *ownedContinuityRuntime {
	return &ownedContinuityRuntime{
		engine:        engine,
		intelligence:  continuity.NewIntelligence(chatSvc),
		semantic:      continuity.NewEmbeddingSemanticMatcher(embeddingSvc),
		disambiguator: continuity.NewLLMThreadDisambiguator(chatSvc),
		spaceID:       spaceID,
		deviceID:      deviceID,
		coreID:        coreID,
	}
}

func (r *ownedContinuityRuntime) request(owner interaction.EntryOwner, roleID, requestID string) business.Request {
	target := owner.TargetDeviceID
	if target == "" {
		target = owner.DeviceID
	}
	return business.Request{
		SpaceID:        owner.SpaceID,
		DeviceID:       owner.DeviceID,
		TargetDeviceID: target,
		CoreID:         owner.CoreID,
		RoleID:         roleID,
		RequestID:      requestID,
	}
}

func (r *ownedContinuityRuntime) Resolve(ctx context.Context, owner interaction.EntryOwner, roleID, conversationID, workspaceID, message, requestID string) (string, error) {
	if r == nil || r.engine == nil || strings.TrimSpace(roleID) == "" {
		return "", errors.New("持续事项解析服务未就绪")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "", nil
	}
	request := r.request(owner, roleID, requestID)
	documents, scope, err := r.engine.ListContinuity(ctx, request, true)
	if err != nil {
		return "", err
	}
	if bound := matchContinuityBinding(documents, conversationID, workspaceID); bound != nil {
		if err := r.bind(ctx, owner, scope, *bound, conversationID, workspaceID, continuityOwnerRequestID(requestID+"|continuity-bind")); err != nil {
			return "", err
		}
		return bound.Thread.ID, nil
	}
	threads := make([]continuity.Thread, 0, len(documents))
	byID := make(map[string]business.OwnedContinuity, len(documents))
	for _, document := range documents {
		if document.ReadOnly || !continuity.ThreadCompatibleCharacter(&document.Thread, roleID) || document.Thread.Status.IsTerminal() {
			continue
		}
		threads = append(threads, document.Thread)
		byID[document.Thread.ID] = document
	}
	semantic := map[string]float64{}
	if len(threads) > 0 && (continuity.IsContinuationCue(message) || continuity.ShouldCreateThread(message)) {
		if scores, scoreErr := r.semantic.Scores(ctx, message, threads); scoreErr == nil {
			semantic = scores
		}
	}
	ranked := continuity.RankThreads(message, threads, semantic)
	if len(ranked) > 0 && ranked[0].Score >= 0.88 {
		document := byID[ranked[0].Thread.ID]
		if err := r.bind(ctx, owner, scope, document, conversationID, workspaceID, continuityOwnerRequestID(requestID+"|continuity-bind")); err != nil {
			return "", err
		}
		return document.Thread.ID, nil
	}
	if len(ranked) > 0 && ranked[0].Score >= 0.55 {
		candidates := make([]continuity.Thread, 0, min(5, len(ranked)))
		for index := 0; index < len(ranked) && index < 5; index++ {
			candidates = append(candidates, ranked[index].Thread)
		}
		threadID, confidence, chooseErr := r.disambiguator.ChooseThread(ctx, message, candidates)
		if chooseErr == nil && confidence >= 0.72 && threadID != "" {
			document := byID[threadID]
			if err := r.bind(ctx, owner, scope, document, conversationID, workspaceID, continuityOwnerRequestID(requestID+"|continuity-bind")); err != nil {
				return "", err
			}
			return threadID, nil
		}
	}
	if !continuity.ShouldCreateThread(message) {
		return "", nil
	}
	createRequest := request
	createRequest.ExpectedScope = &scope
	created, _, err := r.engine.Continuity(ctx, createRequest, business.ContinuityMutation{Action: "create", Title: continuity.DeriveThreadTitle(message), Goal: message, NextAction: "根据本轮回复继续推进", ExpectedRevision: 0})
	if err != nil {
		return "", err
	}
	if err := r.bind(ctx, owner, scope, created, conversationID, workspaceID, continuityOwnerRequestID(requestID+"|continuity-bind")); err != nil {
		return "", err
	}
	return created.Thread.ID, nil
}

func (r *ownedContinuityRuntime) bind(ctx context.Context, owner interaction.EntryOwner, scope coordination.ExecutionScope, document business.OwnedContinuity, conversationID, workspaceID, requestID string) error {
	for _, binding := range []continuity.ThreadBinding{
		continuity.NewThreadBinding(document.Thread.ID, "conversation", conversationID, "context", "resolver", 1),
		continuity.NewThreadBinding(document.Thread.ID, "workspace", workspaceID, "context", "resolver", 1),
	} {
		if binding.BindingID == "" {
			continue
		}
		request := r.request(owner, document.Thread.CharacterID, continuityOwnerRequestID(requestID+"|"+binding.BindingType))
		request.ExpectedScope = &scope
		current, _, err := r.engine.Continuity(ctx, request, business.ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "bind", Binding: &binding})
		if err != nil {
			if errors.Is(err, coordination.ErrResourceVersion) {
				continue
			}
			return err
		}
		document = current
	}
	return nil
}

func matchContinuityBinding(documents []business.OwnedContinuity, conversationID, workspaceID string) *business.OwnedContinuity {
	for index := range documents {
		document := &documents[index]
		if document.ReadOnly || document.Thread.Status.IsTerminal() {
			continue
		}
		if continuity.BindingLookupMatches(document.Bindings, continuity.BindingLookup{Type: "conversation", ID: conversationID}, continuity.BindingLookup{Type: "workspace", ID: workspaceID}) {
			return document
		}
	}
	return nil
}

func (r *ownedContinuityRuntime) Observe(ctx context.Context, owner interaction.EntryOwner, roleID, threadID string, payload continuity.ObservePayload) error {
	if r == nil || r.engine == nil || strings.TrimSpace(threadID) == "" {
		return nil
	}
	payload.ThreadID = threadID
	request := r.request(owner, roleID, continuityOwnerRequestID(payload.RequestID+"|continuity-observe"))
	documents, scope, err := r.engine.ListContinuity(ctx, request, true)
	if err != nil {
		return err
	}
	var document *business.OwnedContinuity
	for index := range documents {
		if documents[index].Thread.ID == threadID {
			document = &documents[index]
			break
		}
	}
	if document == nil {
		return errors.New("持续事项不存在")
	}
	openWaits := make([]continuity.Wait, 0)
	for _, wait := range document.Waits {
		if wait.Status == continuity.WaitStatusWaiting {
			openWaits = append(openWaits, wait)
		}
	}
	patch, extractErr := r.intelligence.Extract(ctx, &document.Thread, openWaits, payload)
	if extractErr != nil {
		heuristic := continuity.BuildHeuristicThreadPatch(&document.Thread, payload)
		patch = heuristic
	}
	request.ExpectedScope = &scope
	_, _, err = r.engine.Continuity(ctx, request, business.ContinuityMutation{ID: threadID, ExpectedRevision: document.Thread.Revision, Action: "observe", Observation: &payload, ThreadPatch: &patch})
	return err
}

func (r *ownedContinuityRuntime) waitForReady(ctx context.Context, max time.Duration) error {
	deadline := time.Now().Add(max)
	for {
		if r != nil && r.engine != nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("持续事项 Owner 服务尚未就绪")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (r *ownedContinuityRuntime) ApplyWorkflow(ctx context.Context, signal continuity.WorkflowLifecycleSignal) error {
	if r == nil || r.engine == nil {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(signal.Status))
	event := business.ContinuityRuntimeEvent{
		Bindings: []continuity.BindingLookup{
			{Type: "workflow_run", ID: signal.ExecutionID},
			{Type: "execution", ID: signal.ExecutionID},
			{Type: "request", ID: signal.OperationID},
			{Type: "conversation", ID: signal.ConversationID},
		},
		EventType:      "workflow." + normalizedContinuityRuntimeEvent(signal.Type, status),
		SourceType:     "workflow",
		SourceID:       signal.ExecutionID,
		ConversationID: signal.ConversationID,
		ExecutionID:    signal.ExecutionID,
		CurrentState:   continuityRuntimeState("工作流执行状态："+status, signal.Error),
		NextAction:     "根据工作流执行结果继续处理",
	}
	event.PayloadJSON = continuityRuntimePayload(map[string]any{"workflowId": signal.WorkflowID, "executionId": signal.ExecutionID, "status": status, "deviceId": signal.DeviceID, "error": signal.Error})
	if strings.Contains(status, "waiting_device") && signal.DeviceID != "" {
		event.ThreadStatus = continuity.ThreadStatusWaiting
		event.Wait = &continuity.Wait{WaitType: continuity.WaitTypeDevice, Description: "等待设备上线以继续工作流", ConditionJSON: continuityRuntimePayload(map[string]any{"deviceId": signal.DeviceID, "executionId": signal.ExecutionID}), ResumeHint: "设备上线后继续工作流", SourceExecutionID: signal.ExecutionID}
	}
	if runtimeTerminalContinuityStatus(status) {
		event.Signal = &continuity.Signal{WaitType: continuity.WaitTypeDependency, Source: "workflow", SourceID: signal.ExecutionID, Attributes: map[string]any{"executionId": signal.ExecutionID, "workflowRunId": signal.ExecutionID, "operationId": signal.OperationID, "status": status}, OccurredAt: signal.Timestamp}
		if status == "succeeded" || status == "completed" {
			event.ThreadStatus = continuity.ThreadStatusActive
			event.CurrentState = "工作流执行已完成"
		} else {
			event.ThreadStatus = continuity.ThreadStatusBlocked
			event.CurrentState = continuityRuntimeState("工作流执行未成功", firstNonEmptyContinuity(signal.Error, status))
			event.NextAction = "评估失败原因并决定重试、修复或调整方案"
		}
	}
	return r.applyRuntimeEvent(ctx, signal.SpaceID, signal.CharacterID, signal.DeviceID, "workflow", signal.ExecutionID, event)
}

func (r *ownedContinuityRuntime) ApplyTask(ctx context.Context, signal continuity.TaskLifecycleSignal) error {
	if r == nil || r.engine == nil {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(signal.Status))
	event := business.ContinuityRuntimeEvent{
		Bindings: []continuity.BindingLookup{
			{Type: "task_run", ID: signal.TaskRunID},
			{Type: "request", ID: signal.OperationID},
			{Type: "execution", ID: signal.InvocationID},
		},
		EventType:    "task." + normalizedContinuityRuntimeEvent(signal.Type, status),
		SourceType:   "task_runtime",
		SourceID:     signal.TaskRunID,
		ExecutionID:  signal.InvocationID,
		CurrentState: continuityRuntimeState("任务执行状态："+status, firstNonEmptyContinuity(signal.Reason, signal.ErrorCode)),
		NextAction:   "根据任务执行结果继续处理",
	}
	event.PayloadJSON = continuityRuntimePayload(map[string]any{"taskRunId": signal.TaskRunID, "operationId": signal.OperationID, "invocationId": signal.InvocationID, "status": status, "deviceId": signal.DeviceID, "reason": signal.Reason, "errorCode": signal.ErrorCode})
	if strings.Contains(status, "waiting_device") && signal.DeviceID != "" {
		event.ThreadStatus = continuity.ThreadStatusWaiting
		event.Wait = &continuity.Wait{WaitType: continuity.WaitTypeDevice, Description: "等待设备上线以继续任务", ConditionJSON: continuityRuntimePayload(map[string]any{"deviceId": signal.DeviceID, "taskRunId": signal.TaskRunID}), ResumeHint: "设备上线后继续任务", SourceExecutionID: signal.TaskRunID}
	}
	if runtimeTerminalContinuityStatus(status) {
		event.Signal = &continuity.Signal{WaitType: continuity.WaitTypeDependency, Source: "task_runtime", SourceID: signal.TaskRunID, Attributes: map[string]any{"taskRunId": signal.TaskRunID, "operationId": signal.OperationID, "invocationId": signal.InvocationID, "status": status}, OccurredAt: signal.Timestamp}
		if status == "succeeded" || status == "completed" {
			event.ThreadStatus = continuity.ThreadStatusActive
			event.CurrentState = "任务执行已完成"
		} else {
			event.ThreadStatus = continuity.ThreadStatusBlocked
			event.CurrentState = continuityRuntimeState("任务执行未成功", firstNonEmptyContinuity(signal.Reason, signal.ErrorCode, status))
			event.NextAction = "评估失败原因并决定重试、修复或调整方案"
		}
	}
	return r.applyRuntimeEvent(ctx, signal.SpaceID, "", signal.DeviceID, "task", signal.TaskRunID, event)
}

func (r *ownedContinuityRuntime) ApplyApprovalRequested(ctx context.Context, signal continuity.ApprovalLifecycleSignal) error {
	if r == nil || r.engine == nil {
		return nil
	}
	event := business.ContinuityRuntimeEvent{
		Bindings:       []continuity.BindingLookup{{Type: "request", ID: signal.RequestID}, {Type: "conversation", ID: signal.ConversationID}},
		EventType:      "approval.requested",
		SourceType:     "approval",
		SourceID:       signal.ApprovalID,
		ConversationID: signal.ConversationID,
		ThreadStatus:   continuity.ThreadStatusWaiting,
		CurrentState:   "等待用户审批后继续",
		NextAction:     "用户审批后继续原计划",
		PayloadJSON:    continuityRuntimePayload(map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID}),
		Wait:           &continuity.Wait{WaitType: continuity.WaitTypeApproval, Description: "等待用户审批后继续", ConditionJSON: continuityRuntimePayload(map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID}), ResumeHint: "审批完成后继续", SourceExecutionID: signal.RequestID},
	}
	return r.applyRuntimeEvent(ctx, signal.SpaceID, "", "", "approval", signal.ApprovalID, event)
}

func (r *ownedContinuityRuntime) ApplyApprovalResolved(ctx context.Context, signal continuity.ApprovalLifecycleSignal) error {
	if r == nil || r.engine == nil {
		return nil
	}
	event := business.ContinuityRuntimeEvent{
		Bindings:       []continuity.BindingLookup{{Type: "request", ID: signal.RequestID}, {Type: "conversation", ID: signal.ConversationID}},
		EventType:      "approval.resolved",
		SourceType:     "approval",
		SourceID:       signal.ApprovalID,
		ConversationID: signal.ConversationID,
		CurrentState:   "审批结果已返回",
		NextAction:     "根据审批结果继续或停止原计划",
		PayloadJSON:    continuityRuntimePayload(map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID, "approved": signal.Approved, "reason": signal.Reason}),
		Signal:         &continuity.Signal{WaitType: continuity.WaitTypeApproval, Source: "approval", SourceID: signal.ApprovalID, Attributes: map[string]any{"approvalId": signal.ApprovalID, "requestId": signal.RequestID, "toolCallId": signal.ToolCallID, "approved": signal.Approved}, OccurredAt: signal.Timestamp},
	}
	return r.applyRuntimeEvent(ctx, signal.SpaceID, "", "", "approval", signal.ApprovalID, event)
}

func (r *ownedContinuityRuntime) ApplyDeviceReady(ctx context.Context, spaceID, deviceID string) error {
	if r == nil || r.engine == nil || strings.TrimSpace(deviceID) == "" {
		return nil
	}
	request := r.request(interaction.EntryOwner{SpaceID: spaceID, DeviceID: r.deviceID, TargetDeviceID: deviceID, CoreID: r.coreID}, "", "device-ready/"+deviceID)
	_, err := r.engine.SignalContinuity(ctx, request, continuity.Signal{WaitType: continuity.WaitTypeDevice, Source: "device_ready", SourceID: deviceID, Attributes: map[string]any{"deviceId": deviceID}, OccurredAt: time.Now().UTC()})
	return err
}

func (r *ownedContinuityRuntime) applyRuntimeEvent(ctx context.Context, spaceID, roleID, deviceID, source, sourceID string, event business.ContinuityRuntimeEvent) error {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		spaceID = r.spaceID
	}
	target := strings.TrimSpace(deviceID)
	if target == "" {
		target = r.deviceID
	}
	owner := interaction.EntryOwner{SpaceID: spaceID, DeviceID: r.deviceID, TargetDeviceID: target, CoreID: r.coreID}
	request := r.request(owner, roleID, "runtime/"+source+"/"+event.EventType+"/"+sourceID)
	_, err := r.engine.ApplyContinuityRuntimeEvent(ctx, request, event)
	return err
}

func runtimeTerminalContinuityStatus(status string) bool {
	switch status {
	case "succeeded", "completed", "failed", "cancelled", "canceled", "interrupted", "timed_out", "timeout", "cancel_timeout", "cancel_failed", "dropped", "compensated", "compensation_failed", "manual_intervention_required", "manual_intervention":
		return true
	default:
		return false
	}
}

func normalizedContinuityRuntimeEvent(kind, status string) string {
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

func continuityRuntimeState(prefix, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return prefix
	}
	return prefix + "：" + detail
}

func continuityRuntimePayload(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func firstNonEmptyContinuity(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func continuityOwnerRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 128 {
		return value
	}
	digest := sha256.Sum256([]byte(value))
	return "continuity/" + hex.EncodeToString(digest[:24])
}
