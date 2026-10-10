package devicemesh

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func (rt *Runtime) PublishOwnedSourceTaskEvent(ctx context.Context, run *task_runtime.TaskRun, _ *task_runtime.TaskDefinition, requestID string, call task_runtime.TaskHostNativeCall) (json.RawMessage, error) {
	return rt.invokeSourceTaskEvent(ctx, run, requestID, call, false)
}

func (rt *Runtime) SourceTaskEventContract(ctx context.Context, run *task_runtime.TaskRun, _ *task_runtime.TaskDefinition, requestID string, call task_runtime.TaskHostNativeCall) (task_runtime.TaskHostEventContract, error) {
	var contract task_runtime.TaskHostEventContract
	encoded, err := rt.invokeSourceTaskEvent(ctx, run, requestID, call, true)
	if err != nil {
		return contract, err
	}
	if json.Unmarshal(encoded, &contract) != nil {
		return contract, errors.New("Source事件契约响应无效")
	}
	return contract, nil
}

func (rt *Runtime) invokeSourceTaskEvent(ctx context.Context, run *task_runtime.TaskRun, requestID string, call task_runtime.TaskHostNativeCall, contractOnly bool) (json.RawMessage, error) {
	scope, owned := coordination.FromContext(ctx)
	if !owned || !contractOnly && (scope.Coordinated || scope.ResourceOwnerID != scope.TargetDeviceID) || contractOnly && !scope.Coordinated || run == nil || run.ExecutionTarget.DeviceID.String() != scope.TargetDeviceID || run.Generation < 1 || run.ExecutionAttemptID == "" {
		return nil, coordination.ErrWrongOwner
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	payload, err := encodeSourceTaskEvent(scope, run, requestID, call, contractOnly)
	if err != nil {
		return nil, err
	}
	result, err := rt.InvokeDeviceHandlerWithRuntimeType(ctx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID), capability.RuntimeTypeTask, "task.host.source-event", payload, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if result.DeviceID != scope.TargetDeviceID || result.RuntimeID != run.ExecutionTarget.RuntimeID.String() || result.RuntimeSessionID != run.ExecutionTarget.RuntimeSessionID.String() || !json.Valid(result.Structured) || len(result.Structured) > 64<<10 {
		return nil, errors.New("Source事件未获得原设备执行连接的精确确认")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), result.Structured...), nil
}

func encodeSourceTaskEvent(scope coordination.ExecutionScope, run *task_runtime.TaskRun, requestID string, call task_runtime.TaskHostNativeCall, contractOnly bool) ([]byte, error) {
	if run == nil || len(call.Payload) == 0 || len(call.Payload) > 64<<10 || !json.Valid(call.Payload) {
		return nil, errors.New("Source事件正文无效或超过上限")
	}
	payloadBytes := append([]byte(nil), call.Payload...)
	call.Payload = nil
	return json.Marshal(task_runtime.TaskHostSourceEventRequest{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: requestID, Call: call, ContractOnly: contractOnly, PayloadBytes: payloadBytes})
}
