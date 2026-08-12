package browser

import "context"

type BrowserRuntime interface {
	Start(ctx context.Context) (*BrowserRuntimeInfo, *BrowserError)
	Stop(ctx context.Context) *BrowserError
	Status(ctx context.Context) BrowserRuntimeInfo
	Health(ctx context.Context) BrowserRuntimeHealth
}

type BrowserEngine interface {
	Start(ctx context.Context) (*BrowserRuntimeInfo, error)
	Stop(ctx context.Context) error
	Status(ctx context.Context) BrowserRuntimeInfo
	Health(ctx context.Context) BrowserRuntimeHealth
}
