package browser

import (
	"context"
	"fmt"
)

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

func (b *chromiumInputBackend) getClient() *cdpClient {
	if b.engine == nil {
		return nil
	}
	return b.engine.cdpClient()
}

func (b *chromiumInputBackend) getSession(targetID TargetID) string {
	if b.engine == nil {
		return ""
	}
	return b.engine.Pages().(*chromiumPageController).getSession(targetID)
}

func (b *chromiumInputBackend) ensureSession(ctx context.Context, targetID TargetID) string {
	if b.engine == nil {
		return ""
	}
	client := b.getClient()
	if client == nil {
		return ""
	}
	return b.engine.Pages().(*chromiumPageController).ensureSession(ctx, client, targetID)
}

func (b *chromiumInputBackend) DispatchMouseMove(ctx context.Context, targetID TargetID, x, y float64) error {
	client := b.getClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.ensureSession(ctx, targetID)
	params := map[string]interface{}{
		"type":       "mouseMoved",
		"x":          x,
		"y":          y,
		"button":     "none",
		"buttons":    0,
		"clickCount": 0,
	}
	if err := client.Call(ctx, "Input.dispatchMouseEvent", sessionID, params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent failed: %w", err)
	}
	return nil
}

func (b *chromiumInputBackend) DispatchMouseDown(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error {
	client := b.getClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.getSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}
	params := map[string]interface{}{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     button,
		"clickCount": clickCount,
	}
	if err := client.Call(ctx, "Input.dispatchMouseEvent", sessionID, params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent failed: %w", err)
	}
	return nil
}

func (b *chromiumInputBackend) DispatchMouseUp(ctx context.Context, targetID TargetID, x, y float64, button string, clickCount int) error {
	client := b.getClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.getSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}
	params := map[string]interface{}{
		"type":       "mouseReleased",
		"x":          x,
		"y":          y,
		"button":     button,
		"clickCount": clickCount,
	}
	if err := client.Call(ctx, "Input.dispatchMouseEvent", sessionID, params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent failed: %w", err)
	}
	return nil
}

func (b *chromiumInputBackend) DispatchMouseWheel(ctx context.Context, targetID TargetID, deltaX, deltaY int64) error {
	client := b.getClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.getSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}
	params := map[string]interface{}{
		"type":   "mouseWheel",
		"x":      0,
		"y":      0,
		"deltaX": deltaX,
		"deltaY": deltaY,
	}
	if err := client.Call(ctx, "Input.dispatchMouseEvent", sessionID, params, nil); err != nil {
		return fmt.Errorf("Input.dispatchMouseEvent failed: %w", err)
	}
	return nil
}

func (b *chromiumInputBackend) InsertText(ctx context.Context, targetID TargetID, text string) error {
	client := b.getClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.getSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}
	params := map[string]interface{}{
		"text": text,
	}
	if err := client.Call(ctx, "Input.insertText", sessionID, params, nil); err != nil {
		return fmt.Errorf("Input.insertText failed: %w", err)
	}
	return nil
}
