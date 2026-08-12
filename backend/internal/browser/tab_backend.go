package browser

import "context"

type BrowserTabBackend interface {
	CreateTarget(ctx context.Context, browserContextID BrowserContextID, initialURL string) (TargetID, error)
	CloseTarget(ctx context.Context, targetID TargetID) error
	ActivateTarget(ctx context.Context, targetID TargetID) error
	TargetInfo(ctx context.Context, targetID TargetID) (TargetInfo, error)
}

type chromiumTabBackend struct {
	engine BrowserTargetController
}

func NewChromiumTabBackend(engine BrowserTargetController) BrowserTabBackend {
	return &chromiumTabBackend{engine: engine}
}

func (b *chromiumTabBackend) CreateTarget(ctx context.Context, browserContextID BrowserContextID, initialURL string) (TargetID, error) {
	return b.engine.CreateTarget(ctx, browserContextID, initialURL)
}

func (b *chromiumTabBackend) CloseTarget(ctx context.Context, targetID TargetID) error {
	return b.engine.CloseTarget(ctx, targetID)
}

func (b *chromiumTabBackend) ActivateTarget(ctx context.Context, targetID TargetID) error {
	return b.engine.ActivateTarget(ctx, targetID)
}

func (b *chromiumTabBackend) TargetInfo(ctx context.Context, targetID TargetID) (TargetInfo, error) {
	return b.engine.TargetInfo(ctx, targetID)
}

type BrowserTargetController interface {
	CreateTarget(ctx context.Context, browserContextID BrowserContextID, initialURL string) (TargetID, error)
	CloseTarget(ctx context.Context, targetID TargetID) error
	ActivateTarget(ctx context.Context, targetID TargetID) error
	TargetInfo(ctx context.Context, targetID TargetID) (TargetInfo, error)
}
