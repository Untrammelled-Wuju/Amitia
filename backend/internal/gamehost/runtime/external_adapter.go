package runtime

import (
	"context"
)

type ExternalServiceAdapter interface {
	Start(ctx context.Context, execCtx ServiceExecutionContext) error
	Stop(ctx context.Context, execCtx ServiceExecutionContext) error
}

type defaultExternalAdapter struct{}

func NewExternalServiceAdapter() ExternalServiceAdapter {
	return &defaultExternalAdapter{}
}

func (a *defaultExternalAdapter) Start(ctx context.Context, execCtx ServiceExecutionContext) error {
	return nil
}

func (a *defaultExternalAdapter) Stop(ctx context.Context, execCtx ServiceExecutionContext) error {
	return nil
}

type ExternalAdapterFunc struct {
	StartFn func(ctx context.Context, execCtx ServiceExecutionContext) error
	StopFn  func(ctx context.Context, execCtx ServiceExecutionContext) error
}

func (f ExternalAdapterFunc) Start(ctx context.Context, execCtx ServiceExecutionContext) error {
	if f.StartFn != nil {
		return f.StartFn(ctx, execCtx)
	}
	return nil
}

func (f ExternalAdapterFunc) Stop(ctx context.Context, execCtx ServiceExecutionContext) error {
	if f.StopFn != nil {
		return f.StopFn(ctx, execCtx)
	}
	return nil
}
