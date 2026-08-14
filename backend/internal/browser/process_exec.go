package browser

import (
	"context"
	"time"

	proc "github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimehost"
)

type browserProcessExec struct {
	engine BrowserEngine
}

func NewBrowserProcessExec(engine BrowserEngine) runtimehost.ProcessExec {
	return &browserProcessExec{engine: engine}
}

func (e *browserProcessExec) Start() (pid int, handle proc.ProcessTreeHandle, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	info, startErr := e.engine.Start(ctx)
	if startErr != nil {
		return 0, 0, startErr
	}
	if info == nil {
		return 0, 0, &BrowserError{Code: ErrCodeBrowserStartFailed, Message: "browser start returned nil info"}
	}

	chromium, ok := e.engine.(*chromiumEngine)
	if !ok {
		return info.PID, 0, nil
	}
	pid, handle = chromium.processInfo()
	if pid == 0 {
		return info.PID, 0, nil
	}
	return pid, handle, nil
}

func (e *browserProcessExec) Stop(handle proc.ProcessTreeHandle, pid int, gracePeriod time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), gracePeriod+5*time.Second)
	defer cancel()
	return e.engine.Stop(ctx)
}

var _ runtimehost.ProcessExec = (*browserProcessExec)(nil)
var _ runtimehost.ProcessStopper = (*browserProcessExec)(nil)
