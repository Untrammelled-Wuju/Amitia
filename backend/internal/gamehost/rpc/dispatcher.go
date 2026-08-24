package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

var _ ipc.Dispatcher = (*RPCDispatcher)(nil)

type RPCDispatcher struct {
	mu            sync.RWMutex
	controlPlane  ipc.ControlPlane
	namespaces    NamespaceRegistry
	hostHandlers  HandlerRegistry
	idGenerator   func() string
	lifecycle     *RequestLifecycleManager
	notifications notification.NotificationBridge
}

type DispatcherConfig struct {
	Namespaces    NamespaceRegistry
	HostHandlers  HandlerRegistry
	IDGenerator   func() string
	Lifecycle     *RequestLifecycleManager
	Notifications notification.NotificationBridge
}

func NewRPCDispatcher(config DispatcherConfig) *RPCDispatcher {
	idGen := config.IDGenerator
	if idGen == nil {
		idGen = defaultIDGenerator()
	}

	dm := config.Lifecycle
	if dm == nil {
		dm = NewLifecycleManager(LifecycleManagerConfig{IDGenerator: idGen})
	}

	return &RPCDispatcher{
		namespaces:    config.Namespaces,
		hostHandlers:  config.HostHandlers,
		idGenerator:   idGen,
		lifecycle:     dm,
		notifications: config.Notifications,
	}
}

func (d *RPCDispatcher) SetControlPlane(cp ipc.ControlPlane) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.controlPlane = cp
	d.lifecycle.SetControlPlane(cp)
}

func (d *RPCDispatcher) SetLifecycleManager(lm *RequestLifecycleManager) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lifecycle = lm
}

func (d *RPCDispatcher) getControlPlane() ipc.ControlPlane {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.controlPlane
}

func (d *RPCDispatcher) getLifecycle() *RequestLifecycleManager {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lifecycle
}

func (d *RPCDispatcher) Dispatch(ctx context.Context, source ipc.DispatchSource, envelope protocol.Envelope) error {
	switch envelope.Type {
	case protocol.MessageTypeRequest:
		return d.dispatchRequest(ctx, source, envelope)
	case protocol.MessageTypeResponse, protocol.MessageTypeError:
		return d.dispatchResponse(source, envelope)
	case protocol.MessageTypeNotification:
		if cancelReq, ok := ParseCancelEnvelope(&envelope); ok {
			dm := d.getLifecycle()
			if dm != nil {
				_ = dm.HandleCancel(source.Peer, cancelReq)
			}
			return nil
		}
		if d.notifications == nil {
			return nil
		}
		return d.notifications.Handle(ctx, notification.RouteContext{
			PluginID:  source.Peer.PluginID,
			RuntimeID: source.Peer.RuntimeID,
			ServiceID: source.Peer.ServiceID,
		}, envelope.Method, envelope.Payload, envelope.Metadata)
	default:
		return nil
	}
}

func (d *RPCDispatcher) dispatchResponse(source ipc.DispatchSource, envelope protocol.Envelope) error {
	dm := d.getLifecycle()
	if dm == nil {
		return nil
	}
	return dm.HandleIncomingResponse(source.Peer, envelope)
}

func (d *RPCDispatcher) dispatchRequest(ctx context.Context, source ipc.DispatchSource, envelope protocol.Envelope) error {
	if err := protocol.ValidateMethod(string(envelope.Method)); err != nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrInvalidArgument,
			"invalid method format",
			err,
		)
	}

	namespace, _, err := ParseMethod(envelope.Method)
	if err != nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrInvalidArgument,
			"failed to parse method",
			err,
		)
	}

	if IsReservedNamespace(namespace) {
		return d.dispatchReserved(ctx, source, envelope, namespace)
	}

	return d.dispatchCustom(ctx, source, envelope, namespace)
}

func (d *RPCDispatcher) dispatchReserved(ctx context.Context, source ipc.DispatchSource, envelope protocol.Envelope, namespace Namespace) error {
	if d.hostHandlers == nil {
		return NewRPCErrorWithCause(
			RPCErrorReservedNamespace,
			domain.ErrUnsupported,
			fmt.Sprintf("no host handler for reserved namespace %q", namespace),
			nil,
		)
	}

	handler, err := d.hostHandlers.Resolve(Method(envelope.Method))
	if err != nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("host handler not found for method %q", envelope.Method),
			err,
		)
	}

	req := RPCRequest{
		ConnectionID: string(source.ConnectionID),
		ID:           envelope.ID,
		PluginID:     domain.PluginID(source.Peer.PluginID),
		RuntimeID:    domain.RuntimeInstanceID(source.Peer.RuntimeID),
		ServiceID:    domain.ServiceID(source.Peer.ServiceID),
		Generation:   source.Peer.Generation,
		Namespace:    namespace,
		Method:       Method(envelope.Method),
		Payload:      cloneRawMessage(envelope.Payload),
	}

	resp, err := handler.Handle(ctx, req)
	if err != nil {
		return err
	}

	env := resp.ToProtocolEnvelope()
	env.ID = d.idGenerator()

	cp := d.getControlPlane()
	if cp == nil {
		return NewRPCErrorWithCause(
			RPCErrorServiceUnavailable,
			domain.ErrRuntimeUnavailable,
			"control plane not available for host response",
			nil,
		)
	}

	return cp.Send(ctx, source.Peer, normalizeEnvelope(env))
}

func (d *RPCDispatcher) dispatchCustom(ctx context.Context, source ipc.DispatchSource, envelope protocol.Envelope, namespace Namespace) error {
	if d.namespaces == nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("namespace registry not available for %q", namespace),
			nil,
		)
	}

	route, err := d.namespaces.Resolve(ctx, domain.RuntimeInstanceID(source.Peer.RuntimeID), Method(envelope.Method))
	if err != nil {
		return err
	}

	cp := d.getControlPlane()
	if cp == nil {
		return NewRPCErrorWithCause(
			RPCErrorServiceUnavailable,
			domain.ErrRuntimeUnavailable,
			"control plane not available",
			nil,
		)
	}

	targetPeer := ipc.Peer{
		PluginID:   route.PluginID,
		RuntimeID:  route.RuntimeID,
		ServiceID:  route.ServiceID,
		Generation: source.Peer.Generation,
	}

	downstreamID := d.idGenerator()
	forwardEnv := envelope
	forwardEnv.ID = downstreamID
	forwardEnv.RequestID = ""
	forwardEnv.PluginID = string(route.PluginID)
	forwardEnv.RuntimeID = string(route.RuntimeID)
	forwardEnv.ServiceID = string(route.ServiceID)
	forwardEnv.Generation = targetPeer.Generation

	if dm := d.getLifecycle(); dm != nil {
		dm.CorrelateForward(source.Peer, envelope.ID, targetPeer, downstreamID)
	}
	if err := cp.Send(ctx, targetPeer, forwardEnv); err != nil {
		if dm := d.getLifecycle(); dm != nil {
			dm.correlation.Remove(RequestKeyFromIPC(envelope.ID, source.Peer))
		}
		return err
	}
	return nil
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if src == nil {
		return nil
	}
	dst := make(json.RawMessage, len(src))
	copy(dst, src)
	return dst
}

func normalizeEnvelope(env protocol.Envelope) protocol.Envelope {
	if env.Protocol == "" {
		env.Protocol = protocol.ProtocolVersion
	}
	return env
}

func defaultIDGenerator() func() string {
	var counter atomic.Uint64
	return func() string {
		return fmt.Sprintf("rpc-resp-%d-%d", time.Now().UnixNano(), counter.Add(1))
	}
}
