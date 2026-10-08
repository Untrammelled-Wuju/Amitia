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
	if g.dispatcher == nil {
		return nil
	}
	handler := g.dispatcher.Resolve(name)
	if handler == nil {
		return nil
	}
	return func(invoke protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		var preflight struct {
			Operation string `json:"operation"`
		}
		rolePreflight := name == "coordination.data" && len(invoke.Input) <= 64<<10 && json.Unmarshal(invoke.Input, &preflight) == nil && preflight.Operation == "roles"
		if !g.active.Load() && !rolePreflight {
			return nil, errors.New("服务提供者切换尚未完成")
		}
		return handler(invoke)
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
