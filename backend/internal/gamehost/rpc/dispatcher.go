package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RPCDispatcher struct {
	mu           sync.RWMutex
	controlPlane ipc.ControlPlane
	namespaces   NamespaceRegistry
	hostHandlers HandlerRegistry
	idGenerator  func() string
}

type DispatcherConfig struct {
	Namespaces   NamespaceRegistry
	HostHandlers HandlerRegistry
	IDGenerator  func() string
}

func NewRPCDispatcher(config DispatcherConfig) *RPCDispatcher {
	idGen := config.IDGenerator
	if idGen == nil {
		idGen = defaultIDGenerator()
	}
	return &RPCDispatcher{
		namespaces:   config.Namespaces,
		hostHandlers: config.HostHandlers,
		idGenerator:  idGen,
	}
}

func (d *RPCDispatcher) SetControlPlane(cp ipc.ControlPlane) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.controlPlane = cp
}

func (d *RPCDispatcher) getControlPlane() ipc.ControlPlane {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.controlPlane
}

func (d *RPCDispatcher) Dispatch(ctx context.Context, peer ipc.Peer, envelope protocol.Envelope) error {
	switch envelope.Type {
	case protocol.MessageTypeRequest:
		return d.dispatchRequest(ctx, peer, envelope)
	case protocol.MessageTypeResponse, protocol.MessageTypeError:
		return nil
	case protocol.MessageTypeNotification:
		return nil
	default:
		return nil
	}
}

func (d *RPCDispatcher) dispatchRequest(ctx context.Context, sourcePeer ipc.Peer, envelope protocol.Envelope) error {
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
		return d.dispatchReserved(ctx, sourcePeer, envelope, namespace)
	}

	return d.dispatchCustom(ctx, sourcePeer, envelope, namespace)
}

func (d *RPCDispatcher) dispatchReserved(ctx context.Context, sourcePeer ipc.Peer, envelope protocol.Envelope, namespace Namespace) error {
	if d.hostHandlers == nil {
		return NewRPCErrorWithCause(
			RPCErrorReservedNamespace,
			domain.ErrUnsupported,
			fmt.Sprintf("no host handler for reserved namespace %q", namespace),
			nil,
		)
	}

	handler, err := d.hostHandlers.Resolve(envelope.Method)
	if err != nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("host handler not found for method %q", envelope.Method),
			err,
		)
	}

	req := RPCRequest{
		ID:        envelope.ID,
		PluginID:  domain.PluginID(sourcePeer.PluginID),
		RuntimeID: domain.RuntimeInstanceID(sourcePeer.RuntimeID),
		ServiceID: domain.ServiceID(sourcePeer.ServiceID),
		Namespace: namespace,
		Method:    envelope.Method,
		Payload:   cloneRawMessage(envelope.Payload),
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

	return cp.Send(ctx, sourcePeer, normalizeEnvelope(env))
}

func (d *RPCDispatcher) dispatchCustom(ctx context.Context, sourcePeer ipc.Peer, envelope protocol.Envelope, namespace Namespace) error {
	if d.namespaces == nil {
		return NewRPCErrorWithCause(
			RPCErrorMethodNotFound,
			domain.ErrNotFound,
			fmt.Sprintf("namespace registry not available for %q", namespace),
			nil,
		)
	}

	route, err := d.namespaces.Resolve(ctx, domain.RuntimeInstanceID(sourcePeer.RuntimeID), envelope.Method)
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

	forwardEnv := envelope
	forwardEnv.PluginID = string(route.PluginID)
	forwardEnv.RuntimeID = string(route.RuntimeID)
	forwayEnv.ServiceID = string(route.ServiceID)

	targetPeer := ipc.Peer{
		PluginID:  route.PluginID,
		RuntimeID: route.RuntimeID,
		ServiceID: route.ServiceID,
	}

	return cp.Send(ctx, targetPeer, forwardEnv)
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
	counter := int64(0)
	return func() string {
		counter++
		return fmt.Sprintf("rpc-resp-%d-%d", time.Now().UnixNano(), counter)
	}
}
