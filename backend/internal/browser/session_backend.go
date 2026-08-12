package browser

import "context"

type BrowserSessionBackend interface {
	CreateBrowserContext(ctx context.Context) (BrowserContextID, error)
	DisposeBrowserContext(ctx context.Context, contextID BrowserContextID) error
}

type BrowserContextController interface {
	CreateBrowserContext(ctx context.Context) (BrowserContextID, error)
	DisposeBrowserContext(ctx context.Context, id BrowserContextID) error
}

type chromiumSessionBackend struct {
	engine BrowserContextController
}

func NewChromiumSessionBackend(engine BrowserContextController) BrowserSessionBackend {
	return &chromiumSessionBackend{engine: engine}
}

func (b *chromiumSessionBackend) CreateBrowserContext(ctx context.Context) (BrowserContextID, error) {
	return b.engine.CreateBrowserContext(ctx)
}

func (b *chromiumSessionBackend) DisposeBrowserContext(ctx context.Context, contextID BrowserContextID) error {
	return b.engine.DisposeBrowserContext(ctx, contextID)
}
