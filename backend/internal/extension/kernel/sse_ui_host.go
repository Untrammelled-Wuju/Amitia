package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/sse"
)

const (
	dialogResponseTimeout = 25 * time.Second
)

type pendingDialog struct {
	resultCh chan string
	errCh    chan error
}

type SSEUIHostNotifier struct {
	hub            *sse.Hub
	mu             sync.Mutex
	pendingDialogs map[string]*pendingDialog
}

func NewSSEUIHostNotifier(hub *sse.Hub) *SSEUIHostNotifier {
	return &SSEUIHostNotifier{
		hub:            hub,
		pendingDialogs: make(map[string]*pendingDialog),
	}
}

func (n *SSEUIHostNotifier) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	if n.hub == nil || !n.hub.HasClients() {
		return ErrUIHostUnavailable
	}
	n.hub.Broadcast("ui_notify", map[string]interface{}{
		"extensionId": extensionID,
		"title":       title,
		"body":        body,
		"severity":    severity,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (n *SSEUIHostNotifier) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	if n.hub == nil || !n.hub.HasClients() {
		return "", ErrDialogHostUnavailable
	}
	if dialogID == "" {
		dialogID = fmt.Sprintf("dialog-%s", uuid.NewString())
	}
	pd := &pendingDialog{
		resultCh: make(chan string, 1),
		errCh:    make(chan error, 1),
	}
	n.mu.Lock()
	n.pendingDialogs[dialogID] = pd
	n.mu.Unlock()

	defer func() {
		n.mu.Lock()
		delete(n.pendingDialogs, dialogID)
		n.mu.Unlock()
	}()

	n.hub.Broadcast("ui_dialog", map[string]interface{}{
		"dialogId":    dialogID,
		"extensionId": extensionID,
		"message":     message,
		"buttons":     buttons,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})

	select {
	case result := <-pd.resultCh:
		return result, nil
	case err := <-pd.errCh:
		return "", err
	case <-time.After(dialogResponseTimeout):
		return "", fmt.Errorf("dialog %s timed out waiting for host response", dialogID)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (n *SSEUIHostNotifier) Navigate(ctx context.Context, extensionID string, target string) error {
	if n.hub == nil || !n.hub.HasClients() {
		return ErrNavigationHostUnavailable
	}
	n.hub.Broadcast("ui_navigate", map[string]interface{}{
		"extensionId": extensionID,
		"target":      target,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (n *SSEUIHostNotifier) ResolveDialog(dialogID string, result string) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case pd.resultCh <- result:
	default:
		return false
	}
	return true
}

func (n *SSEUIHostNotifier) FailDialog(dialogID string, err error) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case pd.errCh <- err:
	default:
		return false
	}
	return true
}

func (n *SSEUIHostNotifier) HasPendingDialog(dialogID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.pendingDialogs[dialogID]
	return ok
}
