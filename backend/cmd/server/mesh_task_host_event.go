package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func registerSourceTaskHostEventDispatcher(dispatcher interface {
	RegisterCancellable(string, agent.CancellableRuntimeInvokeHandler)
}, services *AppServices) {
	dispatcher.RegisterCancellable("task.host.source-event", func(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		if services == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil || len(invoke.Input) > 128<<10 {
			return nil, fmt.Errorf("Source任务事件服务未就绪或请求过大")
		}
		var request task_runtime.TaskHostSourceEventRequest
		if json.Unmarshal(invoke.Input, &request) != nil {
			return nil, fmt.Errorf("Source任务事件请求无效")
		}
		result, err := services.KernelContainer.TaskRuntimeService.PublishSourceTaskNativeEvent(ctx, request)
		if err != nil {
			return nil, err
		}
		return &protocol.RuntimeResultPayload{InvocationID: invoke.InvocationID, Status: "success", Result: result}, nil
	})
}
