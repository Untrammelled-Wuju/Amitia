package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

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

type meshOwnedToolRuntime struct{ services *AppServices }

type ownedToolResultDocument struct {
	Scope       coordination.ExecutionScope  `json:"executionScope"`
	Fingerprint string                       `json:"fingerprint"`
	Result      capability.UnifiedToolResult `json:"result"`
}

func ownedToolScope(inference business.Inference, call string) kernel.InvocationScope {
	scope := inference.Scope
	exec := execution.NewExecutionContext(scope.ExecutionID, scope.SpaceID)
	exec.ExecutionID = scope.ExecutionID
	exec.ConversationID = inference.ConversationID
	exec.RuntimeTarget = &execution.RuntimeTarget{Placement: "device", SpaceID: runtimeidentity.SpaceID(scope.SpaceID), DeviceID: runtimeidentity.DeviceID(scope.TargetDeviceID)}
	exec.Metadata = map[string]any{"deviceMeshScope": scope, "ownedDeviceTarget": true}
	return kernel.InvocationScope{SpaceID: scope.SpaceID, DeviceID: scope.TargetDeviceID, PrincipalType: "device", CharacterID: scope.RoleID, ConversationID: inference.ConversationID, Channel: "device-mesh", RequestID: scope.RequestID, ToolCallID: call, Message: inference.Message, Source: "device-mesh", ExecContext: &exec}
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
	return allowed, nil
}

func (r *meshOwnedToolRuntime) Execute(ctx context.Context, inference business.Inference, call, name string, input json.RawMessage) (chat.ToolResult, error) {
	definition, err := r.definition(ctx, inference, name)
	if err != nil {
		return chat.ToolResult{}, err
	}
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
			return (&chatToolRuntimeAdapter{}).toChatResult(kernel.ToolDispatchResultFromUnified(saved.Result)), nil
		}
		ctx = kernelExecution.WithOwnedToolResultStore(ctx, func(current context.Context, result capability.UnifiedToolResult) (kernelExecution.OwnedToolResultReference, error) {
			return r.saveOwnedToolResult(current, inference.Scope, key, fingerprint, result)
		})
	}
	result, found := r.services.KernelContainer.ToolFacade.ExecuteTool(ctx, capability.CapabilityID(definition.ID), input, ownedToolScope(inference, call), call, key)
	if !found {
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
	return (&chatToolRuntimeAdapter{}).toChatResult(result), nil
}

func (r *meshOwnedToolRuntime) saveOwnedToolResult(ctx context.Context, scope coordination.ExecutionScope, id, fingerprint string, result capability.UnifiedToolResult) (kernelExecution.OwnedToolResultReference, error) {
	encoded, err := json.Marshal(ownedToolResultDocument{Scope: scope, Fingerprint: fingerprint, Result: result})
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
