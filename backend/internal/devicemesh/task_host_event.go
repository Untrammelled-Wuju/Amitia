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
	scope, owned := coordination.FromContext(ctx)
	if !owned || scope.Coordinated || scope.ResourceOwnerID != scope.TargetDeviceID || run == nil || run.ExecutionTarget.DeviceID.String() != scope.TargetDeviceID || run.Generation < 1 || run.ExecutionAttemptID == "" {
		return nil, coordination.ErrWrongOwner
	}
	if err := rt.validateDataRoute(ctx, scope); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(task_runtime.TaskHostSourceEventRequest{Scope: scope, TaskRunID: run.TaskRunID, Generation: run.Generation, AttemptID: run.ExecutionAttemptID.String(), RequestID: requestID, Call: call})
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
