package browser

import "context"

type InputBackend interface {
	DispatchMouseMove(ctx context.Context, targetID TargetID, x, y float64) error
	DispatchMouseDown(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error
	DispatchMouseUp(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error
	DispatchMouseWheel(ctx context.Context, targetID TargetID, deltaX, deltaY int64) error
	InsertText(ctx context.Context, targetID TargetID, text string) error
}

type chromiumInputBackend struct {
	engine *chromiumEngine
}

func NewChromiumInputBackend(engine BrowserEngine) InputBackend {
	if e, ok := engine.(*chromiumEngine); ok {
		return &chromiumInputBackend{engine: e}
	}
	return &chromiumInputBackend{}
}

func (b *chromiumInputBackend) DispatchMouseMove(ctx context.Context, targetID TargetID, x, y float64) error {
	return nil
}

func (b *chromiumInputBackend) DispatchMouseDown(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error {
	return nil
}

func (b *chromiumInputBackend) DispatchMouseUp(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error {
	return nil
}

func (b *chromiumInputBackend) DispatchMouseWheel(ctx context.Context, targetID TargetID, deltaX, deltaY int64) error {
	return nil
}

func (b *chromiumInputBackend) InsertText(ctx context.Context, targetID TargetID, text string) error {
	return nil
}
