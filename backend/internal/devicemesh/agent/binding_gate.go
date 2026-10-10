package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type bindingGate struct {
	active     atomic.Bool
	dispatcher RuntimeDispatcher
	worker     TaskWorkerIface
}

func (g *bindingGate) Resolve(name string) RuntimeInvokeHandler {
	handler := g.ResolveContext(name)
	if handler == nil {
		return nil
	}
	return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		return handler(context.Background(), invoke)
	}
}

func (g *bindingGate) ResolveContext(name string) CancellableRuntimeInvokeHandler {
	if g.dispatcher == nil {
		return nil
	}
	var handler CancellableRuntimeInvokeHandler
	if contextual, ok := g.dispatcher.(RuntimeContextDispatcher); ok {
		handler = contextual.ResolveContext(name)
	} else if legacy := g.dispatcher.Resolve(name); legacy != nil {
		handler = func(_ context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
			return legacy(invoke)
		}
	}
	if handler == nil {
		return nil
	}
	return func(ctx context.Context, invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		if err := ctx.Err(); err != nil {
			return nil, context.Cause(ctx)
		}
		var preflight struct {
			Operation string `json:"operation"`
		}
		rolePreflight := name == "coordination.data" && len(invoke.Input) <= 64<<10 && json.Unmarshal(invoke.Input, &preflight) == nil && preflight.Operation == "roles"
		if !g.active.Load() && !rolePreflight {
			return nil, errors.New("服务提供者切换尚未完成")
		}
		return handler(ctx, invoke)
	}
}

func (g *bindingGate) CancelInvocation(id string) bool {
	if dispatcher, ok := g.dispatcher.(RuntimeCancelDispatcher); ok {
		return dispatcher.CancelInvocation(id)
	}
	return false
}

func (g *bindingGate) CancelAllInvocations(reason string) int {
	if dispatcher, ok := g.dispatcher.(RuntimeDisconnectDispatcher); ok {
		return dispatcher.CancelAllInvocations(reason)
	}
	return 0
}

func (g *bindingGate) ExecuteTask(ctx context.Context, payload protocol.TaskDispatchPayload) error {
	if !g.active.Load() || g.worker == nil {
		return errors.New("服务提供者切换尚未完成")
	}
	return g.worker.ExecuteTask(ctx, payload)
}

func (g *bindingGate) CancelTask(ctx context.Context, runID, attemptID, leaseID string) error {
	if !g.active.Load() || g.worker == nil {
		return errors.New("服务提供者切换尚未完成")
	}
	return g.worker.CancelTask(ctx, runID, attemptID, leaseID)
}

func (g *bindingGate) PauseTask(ctx context.Context, request protocol.TaskPausePayload) error {
	if !g.active.Load() || g.worker == nil {
		return errors.New("服务提供者切换尚未完成，暂停结果不能确认")
	}
	worker, supported := g.worker.(interface {
		PauseTask(context.Context, protocol.TaskPausePayload) error
	})
	if !supported {
		return errors.New("设备任务执行器不支持暂停")
	}
	return worker.PauseTask(ctx, request)
}

func (g *bindingGate) CancelAllTasks() {
	if worker, ok := g.worker.(interface{ CancelAllTasks() }); ok {
		worker.CancelAllTasks()
	}
}
