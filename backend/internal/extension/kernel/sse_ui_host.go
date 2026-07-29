package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/pkg/sse"
)

const (
	dialogResponseTimeout = 25 * time.Second
)

type pendingDialog struct {
	resultCh      chan string
	errCh         chan error
	hostClientID  string
	hostSessionID string
}

type SSEUIHostNotifier struct {
	hub            *sse.Hub
	hostRegistry   *host_registry.HostRegistry
	mu             sync.Mutex
	pendingDialogs map[string]*pendingDialog
}

func NewSSEUIHostNotifier(hub *sse.Hub) *SSEUIHostNotifier {
	return &SSEUIHostNotifier{
		hub:            hub,
		pendingDialogs: make(map[string]*pendingDialog),
	}
}

func NewSSEUIHostNotifierWithRegistry(hub *sse.Hub, registry *host_registry.HostRegistry) *SSEUIHostNotifier {
	return &SSEUIHostNotifier{
		hub:            hub,
		hostRegistry:   registry,
		pendingDialogs: make(map[string]*pendingDialog),
	}
}

func (n *SSEUIHostNotifier) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	if n.hostRegistry != nil {
		target, err := n.hostRegistry.FindTargetHost(ctx, "", host_registry.CapUINotify, "", "")
		if err != nil {
			return err
		}
		if target == nil {
			return ErrUIHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(target.HostClientID) {
			return ErrUIHostUnavailable
		}
		payload := map[string]interface{}{
			"title":    title,
			"body":     body,
			"severity": severity,
		}
		envelope := NewEventEnvelope("ui_notify", extensionID, payload, defaultEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = target.HostClientID
		envelopeMap["hostSessionId"] = target.HostSessionID
		n.hub.SendToClient(target.HostClientID, "ui_notify", envelopeMap)
		return nil
	}

	if n.hub == nil || !n.hub.HasClients() {
		return ErrUIHostUnavailable
	}
	payload := map[string]interface{}{
		"title":    title,
		"body":     body,
		"severity": severity,
	}
	envelope := NewEventEnvelope("ui_notify", extensionID, payload, defaultEventTTL)
	n.hub.Broadcast("ui_notify", envelope.ToMap())
	return nil
}

func (n *SSEUIHostNotifier) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	if dialogID == "" {
		dialogID = fmt.Sprintf("dialog-%s", uuid.NewString())
	}

	pd := &pendingDialog{
		resultCh: make(chan string, 1),
		errCh:    make(chan error, 1),
	}

	if n.hostRegistry != nil {
		target, err := n.hostRegistry.FindTargetHost(ctx, "", host_registry.CapUIDialog, "", "")
		if err != nil {
			return "", err
		}
		if target == nil {
			return "", ErrDialogHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(target.HostClientID) {
			return "", ErrDialogHostUnavailable
		}
		pd.hostClientID = target.HostClientID
		pd.hostSessionID = target.HostSessionID

		n.mu.Lock()
		n.pendingDialogs[dialogID] = pd
		n.mu.Unlock()

		payload := map[string]interface{}{
			"dialogId": dialogID,
			"message":  message,
			"buttons":  buttons,
		}
		envelope := NewEventEnvelope("ui_dialog", extensionID, payload, dialogEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = target.HostClientID
		envelopeMap["hostSessionId"] = target.HostSessionID
		n.hub.SendToClient(target.HostClientID, "ui_dialog", envelopeMap)
	} else {
		if n.hub == nil || !n.hub.HasClients() {
			return "", ErrDialogHostUnavailable
		}
		n.mu.Lock()
		n.pendingDialogs[dialogID] = pd
		n.mu.Unlock()

		payload := map[string]interface{}{
			"dialogId": dialogID,
			"message":  message,
			"buttons":  buttons,
		}
		envelope := NewEventEnvelope("ui_dialog", extensionID, payload, dialogEventTTL)
		n.hub.Broadcast("ui_dialog", envelope.ToMap())
	}

	defer func() {
		n.mu.Lock()
		delete(n.pendingDialogs, dialogID)
		n.mu.Unlock()
	}()

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
	if n.hostRegistry != nil {
		host, err := n.hostRegistry.FindTargetHost(ctx, "", host_registry.CapUINavigate, "", "")
		if err != nil {
			return err
		}
		if host == nil {
			return ErrNavigationHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(host.HostClientID) {
			return ErrNavigationHostUnavailable
		}
		payload := map[string]interface{}{
			"target": target,
		}
		envelope := NewEventEnvelope("ui_navigate", extensionID, payload, defaultEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = host.HostClientID
		envelopeMap["hostSessionId"] = host.HostSessionID
		n.hub.SendToClient(host.HostClientID, "ui_navigate", envelopeMap)
		return nil
	}

	if n.hub == nil || !n.hub.HasClients() {
		return ErrNavigationHostUnavailable
	}
	payload := map[string]interface{}{
		"target": target,
	}
	envelope := NewEventEnvelope("ui_navigate", extensionID, payload, defaultEventTTL)
	n.hub.Broadcast("ui_navigate", envelope.ToMap())
	return nil
}

func (n *SSEUIHostNotifier) ResolveDialog(dialogID string, result string) bool {
	return n.ResolveDialogWithHost(dialogID, "", "", result)
}

func (n *SSEUIHostNotifier) ResolveDialogWithHost(dialogID string, hostClientID string, hostSessionID string, result string) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	if pd.hostSessionID != "" && pd.hostSessionID != hostSessionID {
		return false
	}
	if pd.hostClientID != "" && pd.hostClientID != hostClientID {
		return false
	}
	select {
	case pd.resultCh <- result:
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) FailDialog(dialogID string, err error) bool {
	return n.FailDialogWithHost(dialogID, "", "", err)
}

func (n *SSEUIHostNotifier) FailDialogWithHost(dialogID string, hostClientID string, hostSessionID string, err error) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	if pd.hostSessionID != "" && pd.hostSessionID != hostSessionID {
		return false
	}
	if pd.hostClientID != "" && pd.hostClientID != hostClientID {
		return false
	}
	select {
	case pd.errCh <- err:
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) FailAllPendingDialogs(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, pd := range n.pendingDialogs {
		select {
		case pd.errCh <- err:
		default:
		}
	}
}

func (n *SSEUIHostNotifier) HasPendingDialog(dialogID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.pendingDialogs[dialogID]
	return ok
}
