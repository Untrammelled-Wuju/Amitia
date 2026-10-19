package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	kernelExecution "github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type meshOwnedToolRuntime struct {
	services *AppServices
	native   *chatMultiAgentRuntime
	parentMu sync.Mutex
}

type ownedToolResultDocument struct {
	ConversationID string                       `json:"conversationId,omitempty"`
	Scope          coordination.ExecutionScope  `json:"executionScope"`
	Fingerprint    string                       `json:"fingerprint"`
	Result         capability.UnifiedToolResult `json:"result"`
}

func ownedToolScope(inference business.Inference, call string) kernel.InvocationScope {
	scope := inference.Scope
	exec := execution.NewExecutionContext(scope.ExecutionID, scope.SpaceID)
	exec.ExecutionID = scope.ExecutionID
	exec.ConversationID = inference.ConversationID
	exec.PrimaryWorkspaceID = inference.PrimaryWorkspaceID
	exec.WorkspaceID = inference.PrimaryWorkspaceID
	exec.WorkspaceBindings = make([]execution.WorkspaceBinding, 0, len(inference.Workspaces))
	for _, workspace := range inference.Workspaces {
		exec.WorkspaceBindings = append(exec.WorkspaceBindings, execution.WorkspaceBinding{ID: workspace.ID, DeviceID: workspace.DeviceID, Name: workspace.Name, Kind: workspace.Kind, RootURI: workspace.RootURI, ReadOnly: workspace.ReadOnly})
		if workspace.ID == inference.PrimaryWorkspaceID {
			exec.Metadata = map[string]any{"workspaceId": workspace.ID, "workspaceDeviceId": workspace.DeviceID, "workspaceName": workspace.Name, "workspaceKind": workspace.Kind, "workspaceRootUri": workspace.RootURI}
		}
	}
	exec.RuntimeTarget = &execution.RuntimeTarget{Placement: "device", SpaceID: runtimeidentity.SpaceID(scope.SpaceID), DeviceID: runtimeidentity.DeviceID(scope.TargetDeviceID)}
	if exec.Metadata == nil {
		exec.Metadata = map[string]any{}
	}
	exec.Metadata["deviceMeshScope"] = scope
	exec.Metadata["ownedDeviceTarget"] = true
	exec.Metadata["projectId"] = inference.ProjectID
	permissionMode := strings.TrimSpace(inference.PermissionMode)
	if permissionMode == "" {
		permissionMode = "request_approval"
	}
	invocation := kernel.InvocationScope{SpaceID: scope.SpaceID, DeviceID: scope.TargetDeviceID, PrincipalType: "device", CharacterID: scope.RoleID, ConversationID: inference.ConversationID, Channel: "device-mesh", RequestID: scope.RequestID, ToolCallID: call, PermissionMode: permissionMode, Message: inference.Message, Source: "owned", ExecContext: &exec}
	if inference.Ingress != nil {
		invocation.Source, invocation.Channel, invocation.SessionID, invocation.IsInternal = inference.Ingress.Source, inference.Ingress.Channel, inference.Ingress.SessionID, inference.Ingress.IsInternal
	}
	return invocation
}

func (r *meshOwnedToolRuntime) definition(ctx context.Context, inference business.Inference, name string) (capability.ToolDefinition, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return capability.ToolDefinition{}, err
	}
	if r.services == nil || r.services.DeviceMesh == nil || r.services.KernelContainer == nil || r.services.KernelContainer.ToolRegistry == nil || r.services.KernelContainer.ToolFacade == nil {
		return capability.ToolDefinition{}, errors.New("Core 能力调用端口未就绪")
	}
	mesh := r.services.DeviceMesh
	if err := mesh.Coordination.Validate(ctx, inference.Scope); err != nil {
		return capability.ToolDefinition{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, mesh, inference.Scope); err != nil {
		return capability.ToolDefinition{}, err
	}
	definition, ok := r.services.KernelContainer.ToolRegistry.GetByModelName(ctx, name)
	if !ok || !definition.Enabled || definition.Internal {
		return capability.ToolDefinition{}, errors.New("能力不存在、未启用或不允许模型调用")
	}
	return r.authorizeDefinition(ctx, inference, definition)
}

func (r *meshOwnedToolRuntime) authorizeDefinition(ctx context.Context, inference business.Inference, definition capability.ToolDefinition) (capability.ToolDefinition, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return capability.ToolDefinition{}, err
	}
	mesh := r.services.DeviceMesh
	if err := mesh.Coordination.Validate(ctx, inference.Scope); err != nil {
		return capability.ToolDefinition{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, mesh, inference.Scope); err != nil {
		return capability.ToolDefinition{}, err
	}
	if !definition.Enabled {
		return capability.ToolDefinition{}, errors.New("能力未启用")
	}
	capabilityID := string(definition.CapabilityID)
	if capabilityID == "" {
		capabilityID = definition.ID
	}
	if err := mesh.Coordination.RequireCapability(ctx, inference.Scope.SpaceID, inference.Scope.InitiatorDeviceID, inference.Scope.TargetDeviceID, capabilityID); err != nil {
		return capability.ToolDefinition{}, err
	}
	if !inference.Scope.Coordinated && (definition.Runtime.RuntimeType == capability.RuntimeTypeInternal || definition.Runtime.RuntimeType == capability.RuntimeTypeTask || definition.Runtime.RuntimeType == capability.RuntimeTypeWorkflow) {
		aware, _ := definition.Metadata["deviceMeshOwnershipAware"].(bool)
		if !aware {
			return capability.ToolDefinition{}, errors.New("该能力尚未适配设备数据归属，拒绝向 Core 的本地数据写入")
		}
	}
	return definition, nil
}

func (r *meshOwnedToolRuntime) Tools(ctx context.Context, inference business.Inference) ([]tool.Tool, error) {
	if r.services == nil || r.services.KernelContainer == nil || r.services.KernelContainer.ToolFacade == nil {
		return nil, errors.New("Core 能力调用端口未就绪")
	}
	available, err := r.services.KernelContainer.ToolFacade.ModelTools(ctx, ownedToolScope(inference, ""))
	if err != nil {
		return nil, err
	}
	allowed := make([]tool.Tool, 0, len(available))
	for _, definition := range available {
		if _, err := r.definition(ctx, inference, definition.Function.Name); err == nil {
			allowed = append(allowed, definition)
		}
	}
	for _, definition := range available {
		if isChatMultiAgentTool(definition.Function.Name) {
			return nil, errors.New("原生任务工具名称与已注册能力冲突")
		}
	}
	if r.native != nil {
		allowed = append(allowed, r.native.ModelTools(ctx, r.nativeScope(ctx, inference))...)
	}
	return allowed, nil
}

func (r *meshOwnedToolRuntime) Execute(ctx context.Context, inference business.Inference, call, name string, input json.RawMessage) (output chat.ToolResult, executionErr error) {
	if err := emitOwnedToolEvent(inference, "tool-started", call, name, input, chat.ToolResult{}, nil); err != nil {
		return chat.ToolResult{}, err
	}
	defer func() {
		if err := emitOwnedToolEvent(inference, "tool-completed", call, name, input, output, executionErr); err != nil {
			executionErr = errors.Join(executionErr, err)
		}
	}()
	if isChatMultiAgentTool(name) {
		if r.native == nil || inference.InternalWorker {
			return chat.ToolResult{}, coordination.ErrWrongOwner
		}
		if err := r.services.OwnedBusiness.WithExecutionScope(ctx, inference.Scope, func(context.Context) error { return nil }); err != nil {
			return chat.ToolResult{}, err
		}
		result := r.native.Execute(ctx, name, input, r.nativeScope(ctx, inference))
		if result.Error == nil && inference.Emit != nil {
			var task business.TaskEvent
			if json.Unmarshal(result.Output, &task) == nil && task.CoordinationID != "" {
				if err := inference.Emit(business.Event{Type: "task", Task: &task}); err != nil {
					return result, err
				}
			}
		}
		return result, nil
	}
	definition, err := r.definition(ctx, inference, name)
	if err != nil {
		return chat.ToolResult{}, err
	}
	return r.executeDefinition(ctx, inference, call, input, definition)
}

func emitOwnedToolEvent(inference business.Inference, eventType, call, name string, input json.RawMessage, result chat.ToolResult, executionErr error) error {
	if inference.Emit == nil {
		return nil
	}
	content:=result.VisibleText
	if len(result.Output)>0 {content=string(result.Output)}
	payload := map[string]any{"toolCallId": call, "toolName": name, "arguments": string(input), "result": content, "durationMs": result.DurationMS,"status":result.Status,"forceVoice":result.ForceVoice}
	if result.Error != nil {
		payload["errorCode"], payload["error"] = result.Error.Code, result.Error.Message
	}
	if eventType=="tool-completed" && result.Error==nil && result.Status!="" && !strings.EqualFold(result.Status,"SUCCESS") && !strings.EqualFold(result.Status,"SUCCEEDED") {payload["errorCode"]="TOOL_"+strings.ToUpper(result.Status);payload["error"]="工具未成功完成"}
	if executionErr != nil {
		payload["error"] = executionErr.Error()
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return inference.Emit(business.Event{Type: eventType, Progress: encoded})
}

func (r *meshOwnedToolRuntime) executeDefinition(ctx context.Context, inference business.Inference, call string, input json.RawMessage, definition capability.ToolDefinition) (chat.ToolResult, error) {
	digest := sha256.Sum256([]byte(inference.Scope.CoreID + "\x00" + inference.Scope.ExecutionID + "\x00" + call))
	key := "owned/" + hex.EncodeToString(digest[:])
	fingerprint := kernelExecution.BuildRequestFingerprintSHA(input, definition.ToolVersion, 0)
	if !inference.Scope.Coordinated {
		resource, err := r.services.DeviceMesh.Resource(ctx, inference.Scope, "tool-result", key)
		if err != nil {
			return chat.ToolResult{}, err
		}
		if resource != nil {
			var saved ownedToolResultDocument
			if resource.Deleted || resource.OwnerID != inference.Scope.ResourceOwnerID || resource.RoleID != inference.Scope.RoleID || json.Unmarshal(resource.Body, &saved) != nil || saved.Fingerprint != fingerprint || saved.Scope != inference.Scope {
				return chat.ToolResult{}, coordination.ErrRequestConflict
			}
			return kernelToolResultToChat(kernel.ToolDispatchResultFromUnified(saved.Result)), nil
		}
		ctx = kernelExecution.WithOwnedToolResultStore(ctx, func(current context.Context, result capability.UnifiedToolResult) (kernelExecution.OwnedToolResultReference, error) {
			return r.saveOwnedToolResult(current, inference.Scope, key, fingerprint, result, inference.ConversationID)
		})
	}
	result, _, executionErr := r.services.KernelContainer.ToolFacade.ExecuteModelToolStream(ctx, definition.ID, input, ownedToolScope(inference, call), key, ownedToolProgressSink{inference: inference, call: call, name: definition.ModelName})
	if executionErr != nil {
		return chat.ToolResult{}, executionErr
	}
	if result.Error != nil && result.Error.Code == "TOOL_NOT_FOUND" {
		return chat.ToolResult{}, errors.New("Core 无法解析所选能力，未执行替代能力")
	}
	if err := r.services.DeviceMesh.Coordination.Validate(ctx, inference.Scope); err != nil {
		return chat.ToolResult{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, r.services.DeviceMesh, inference.Scope); err != nil {
		return chat.ToolResult{}, err
	}
	var reference kernelExecution.OwnedToolResultReference
	if json.Unmarshal(result.Output, &reference) == nil && reference.OwnedToolResult {
		if inference.Scope.Coordinated || reference.OwnerID != inference.Scope.ResourceOwnerID || reference.ResourceID != key {
			return chat.ToolResult{}, coordination.ErrWrongOwner
		}
		resource, err := r.services.DeviceMesh.Resource(ctx, inference.Scope, "tool-result", key)
		if err != nil {
			return chat.ToolResult{}, err
		}
		if resource == nil || resource.Deleted || resource.OwnerID != reference.OwnerID || resource.RoleID != inference.Scope.RoleID {
			return chat.ToolResult{}, coordination.ErrWrongOwner
		}
		hash := sha256.Sum256(resource.Body)
		var saved ownedToolResultDocument
		if hex.EncodeToString(hash[:]) != reference.SHA256 || json.Unmarshal(resource.Body, &saved) != nil || saved.Scope != inference.Scope || saved.Fingerprint != fingerprint {
			return chat.ToolResult{}, errors.New("设备工具结果完整性校验失败")
		}
		result = kernel.ToolDispatchResultFromUnified(saved.Result)
	}
	return kernelToolResultToChat(result), nil
}

func (r *meshOwnedToolRuntime) saveOwnedToolResult(ctx context.Context, scope coordination.ExecutionScope, id, fingerprint string, result capability.UnifiedToolResult, conversation ...string) (kernelExecution.OwnedToolResultReference, error) {
	conversationID := ""
	if len(conversation) > 0 {
		conversationID = conversation[0]
	}
	encoded, err := json.Marshal(ownedToolResultDocument{Scope: scope, Fingerprint: fingerprint, Result: result, ConversationID: conversationID})
	if err != nil {
		return kernelExecution.OwnedToolResultReference{}, err
	}
	scope.RequestID = id + "/result"
	dependencies, err := coordination.CommitDependencies(ctx)
	if err != nil {
		return kernelExecution.OwnedToolResultReference{}, err
	}
	commit := coordination.Commit{Scope: scope, LeaseProof: coordination.CommitLease(ctx), Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: "tool-result", ID: id, RoleID: scope.RoleID, Body: encoded}}}
	ack, err := r.services.DeviceMesh.Commit(ctx, commit)
	if err != nil {
		if errors.Is(err, coordination.ErrRequestConflict) || errors.Is(err, coordination.ErrResourceVersion) || errors.Is(err, coordination.ErrWrongOwner) || errors.Is(err, coordination.ErrScopeExpired) || errors.Is(err, coordination.ErrRoleRequired) {
			return kernelExecution.OwnedToolResultReference{}, err
		}
		if queueErr := r.services.DeviceMesh.Coordination.Enqueue(ctx, commit); queueErr != nil {
			return kernelExecution.OwnedToolResultReference{}, errors.Join(err, queueErr)
		}
		return kernelExecution.OwnedToolResultReference{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || ack.Versions["tool-result/"+id] != 1 {
		return kernelExecution.OwnedToolResultReference{}, errors.New("工具结果所有者尚未确认保存")
	}
	digest := sha256.Sum256(encoded)
	return kernelExecution.OwnedToolResultReference{OwnedToolResult: true, OwnerID: ack.OwnerID, ResourceID: id, SHA256: hex.EncodeToString(digest[:])}, nil
}

var _ chat.OwnedToolRuntime = (*meshOwnedToolRuntime)(nil)
