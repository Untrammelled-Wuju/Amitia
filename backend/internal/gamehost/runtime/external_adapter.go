package runtime

import (
	"context"
)

type ExternalServiceAdapter interface {
	Start(ctx context.Context, execCtx ServiceExecutionContext) error
	Stop(ctx context.Context, execCtx ServiceExecutionContext) error
}

type unavailableExternalServiceAdapter struct{}

func NewUnavailableExternalServiceAdapter() ExternalServiceAdapter {
	return &unavailableExternalServiceAdapter{}
}

func (a *unavailableExternalServiceAdapter) Start(
	ctx context.Context,
	execCtx ServiceExecutionContext,
) error {
	return &ExecutionError{
		Code:      ErrServiceUnavailable,
		RuntimeID: string(execCtx.RuntimeID),
		PluginID:  string(execCtx.PluginID),
		ServiceID: string(execCtx.ServiceID),
		Message:   "external service adapter is not configured",
	}
}

func (a *unavailableExternalServiceAdapter) Stop(
	ctx context.Context,
	execCtx ServiceExecutionContext,
) error {
	return &ExecutionError{
		Code:      ErrServiceUnavailable,
		RuntimeID: string(execCtx.RuntimeID),
		PluginID:  string(execCtx.PluginID),
		ServiceID: string(execCtx.ServiceID),
		Message:   "external service adapter is not configured",
	}
}

type ExternalAdapterFunc struct {
	StartFn func(ctx context.Context, execCtx ServiceExecutionContext) error
	StopFn  func(ctx context.Context, execCtx ServiceExecutionContext) error
}

func (f ExternalAdapterFunc) Start(ctx context.Context, execCtx ServiceExecutionContext) error {
	if f.StartFn != nil {
		return f.StartFn(ctx, execCtx)
	}
	return &ExecutionError{
		Code:      ErrServiceUnavailable,
		RuntimeID: string(execCtx.RuntimeID),
		PluginID:  string(execCtx.PluginID),
		ServiceID: string(execCtx.ServiceID),
		Message:   "external service adapter is not configured",
	}
}

func (f ExternalAdapterFunc) Stop(ctx context.Context, execCtx ServiceExecutionContext) error {
	if f.StopFn != nil {
		return f.StopFn(ctx, execCtx)
	}
	return nil
}
