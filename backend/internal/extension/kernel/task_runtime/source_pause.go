package task_runtime

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
)

type sourceTaskProcess struct {
	dispatch   protocol.TaskDispatchPayload
	host       *TaskProcessHost
	ctx        context.Context
	checkpoint bool
}

func (s *TaskRuntimeService) PauseOwnedSourceDispatch(ctx context.Context, request protocol.TaskPausePayload) error {
	value, exists := s.sourceHosts.Load(request.TaskRunID)
	if !exists {
		return NewTaskError(ErrTaskPauseUnsupported, "设备任务进程尚未就绪或已经退出")
	}
	binding := value.(*sourceTaskProcess)
	if !binding.checkpoint {
		return NewTaskError(ErrTaskPauseUnsupported, "设备任务定义不支持检查点暂停")
	}
	dispatch := binding.dispatch
	if request.AttemptID == "" || request.LeaseID == "" || request.RuntimeSessionID == "" || request.ConnectionGeneration < 1 || dispatch.AttemptID != request.AttemptID || dispatch.LeaseID != request.LeaseID || dispatch.RuntimeSessionID != request.RuntimeSessionID || dispatch.ConnectionGeneration != request.ConnectionGeneration {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备任务暂停请求与当前执行租约不一致")
	}
	if err := coordination.ValidateCurrent(binding.ctx); err != nil {
		return err
	}
	_, err := binding.host.Pause(ctx)
	return err
}
