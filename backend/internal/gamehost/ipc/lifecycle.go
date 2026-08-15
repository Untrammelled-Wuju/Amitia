package ipc

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type ControlPlane interface {
	Attach(
		ctx context.Context,
		peer Peer,
		transport Transport,
	) (*Connection, error)

	Detach(
		ctx context.Context,
		connectionID ConnectionID,
	) error

	Send(
		ctx context.Context,
		peer Peer,
		envelope protocol.Envelope,
	) error

	SendRequest(
		ctx context.Context,
		peer Peer,
		envelope protocol.Envelope,
		timeout time.Duration,
	) (*protocol.Envelope, error)

	Shutdown(ctx context.Context) error
}

type ConnectionHandler interface {
	OnAttach(conn *Connection)

	OnDetach(conn *Connection)

	OnEnvelope(conn *Connection, envelope protocol.Envelope)

	OnError(conn *Connection, err error)
}

type defaultConnectionHandler struct {
	dispatcher Dispatcher
	handler    EventHandler
}

func newDefaultConnectionHandler(dispatcher Dispatcher, handler EventHandler) *defaultConnectionHandler {
	return &defaultConnectionHandler{
		dispatcher: dispatcher,
		handler:    handler,
	}
}

func (h *defaultConnectionHandler) OnAttach(conn *Connection) {
	if h.handler != nil {
		h.handler(ConnectionEvent{
			Type:         EventConnectionAttached,
			ConnectionID: conn.ID,
			Peer:         conn.Peer,
		})
	}
}

func (h *defaultConnectionHandler) OnDetach(conn *Connection) {
	if h.handler != nil {
		h.handler(ConnectionEvent{
			Type:         EventConnectionDetached,
			ConnectionID: conn.ID,
			Peer:         conn.Peer,
		})
	}
}

func (h *defaultConnectionHandler) OnEnvelope(conn *Connection, envelope protocol.Envelope) {
	if h.dispatcher == nil {
		return
	}
	ctx := context.Background()
	source := DispatchSource{
		ConnectionID: conn.ID,
		Peer:         conn.Peer,
	}
	_ = h.dispatcher.Dispatch(ctx, source, envelope)
}

func (h *defaultConnectionHandler) OnError(conn *Connection, err error) {
	if h.handler != nil {
		h.handler(ConnectionEvent{
			Type:         EventConnectionError,
			ConnectionID: conn.ID,
			Peer:         conn.Peer,
			Error:        err,
		})
	}
}

type IDGenerator interface {
	Generate() ConnectionID
}

type ResponseCorrelator interface {
	RegisterPending(peer Peer, requestID string) (respCh chan *protocol.Envelope, cancel func(), ok bool)
	HandleResponse(peer Peer, envelope *protocol.Envelope) bool
	CancelByPeer(peer Peer)
}

type ControlPlaneConfig struct {
	Registry            *ConnectionRegistry
	Resolver            RuntimePeerResolver
	Dispatcher          Dispatcher
	EventHandler        EventHandler
	IDGenerator         IDGenerator
	MaxEnvelopeSize     int64
	HandshakeController HandshakeController
	ResponseCorrelator  ResponseCorrelator
}

type controlPlane struct {
	registry            *ConnectionRegistry
	resolver            RuntimePeerResolver
	dispatcher          Dispatcher
	handler             EventHandler
	connHandler         ConnectionHandler
	idGenerator         IDGenerator
	mu                  sync.RWMutex
	connections         []*Connection
	controlCtx          context.Context
	controlCancel       context.CancelFunc
	shuttingDown        bool
	maxEnvelopeSize     int64
	handshakeController HandshakeController
	responseCorrelator  ResponseCorrelator
}

func NewControlPlane(config ControlPlaneConfig) (ControlPlane, error) {
	if config.Registry == nil {
		return nil, NewIPCError(IPCErrorPeerRoute, domain.ErrInternal, "connection registry is required")
	}
	if config.Resolver == nil {
		return nil, NewIPCError(IPCErrorPeerRoute, domain.ErrInternal, "runtime peer resolver is required")
	}
	if config.Dispatcher == nil {
		return nil, NewIPCError(IPCErrorPeerRoute, domain.ErrInternal, "dispatcher is required")
	}
	if config.HandshakeController == nil {
		return nil, NewIPCError(IPCErrorPeerRoute, domain.ErrInternal, "handshake controller is required")
	}
	if config.IDGenerator == nil {
		config.IDGenerator = NewUUIDIDGenerator()
	}
	if config.MaxEnvelopeSize <= 0 {
		config.MaxEnvelopeSize = defaultMaxEnvelopeSize
	}

	controlCtx, controlCancel := context.WithCancel(context.Background())

	cp := &controlPlane{
		registry:            config.Registry,
		resolver:            config.Resolver,
		dispatcher:          config.Dispatcher,
		handler:             config.EventHandler,
		idGenerator:         config.IDGenerator,
		controlCtx:          controlCtx,
		controlCancel:       controlCancel,
		maxEnvelopeSize:     config.MaxEnvelopeSize,
		handshakeController: config.HandshakeController,
		responseCorrelator:  config.ResponseCorrelator,
	}
	cp.connHandler = newDefaultConnectionHandler(cp.dispatcher, cp.handler)
	return cp, nil
}

func (cp *controlPlane) Attach(ctx context.Context, peer Peer, transport Transport) (*Connection, error) {
	if err := peer.Validate(); err != nil {
		return nil, NewIPCErrorWithCause(IPCErrorPeerRoute, domain.ErrInvalidArgument, "peer validation failed", err)
	}

	cp.mu.Lock()
	if cp.shuttingDown {
		cp.mu.Unlock()
		return nil, NewIPCError(IPCErrorTransport, domain.ErrInvalidState, "control plane is shutting down")
	}
	cp.mu.Unlock()

	pluginID, err := cp.resolver.ResolveService(ctx, peer.RuntimeID, peer.ServiceID)
	if err != nil {
		return nil, NewIPCErrorWithCause(IPCErrorPeerRoute, domain.ErrNotFound, "failed to resolve runtime service", err)
	}

	if pluginID != peer.PluginID {
		return nil, NewIPCErrorWithCause(
			IPCErrorPeerRoute,
			domain.ErrInvalidArgument,
			"plugin id mismatch: peer does not belong to resolved plugin",
			nil,
		)
	}

	if cp.registry.PeerExists(peer.Key()) {
		return nil, NewIPCErrorWithCause(
			IPCErrorDuplicate,
			domain.ErrAlreadyExists,
			"active connection already exists for peer",
			nil,
		)
	}

	now := time.Now().UTC()
	connCtx, connCancel := context.WithCancel(cp.controlCtx)

	id := cp.idGenerator.Generate()
	conn := newConnection(id, peer, transport, now, connCancel)

	if err := cp.registry.Register(conn); err != nil {
		connCancel()
		return nil, NewIPCErrorWithCause(IPCErrorDuplicate, domain.ErrAlreadyExists, "failed to register connection", err)
	}

	cp.mu.Lock()
	cp.connections = append(cp.connections, conn)
	cp.mu.Unlock()

	cp.handshakeController.Register(conn.ID)

	cp.connHandler.OnAttach(conn)

	go cp.receiveLoop(conn, connCtx)

	return conn, nil
}

func (cp *controlPlane) Detach(ctx context.Context, connectionID ConnectionID) error {
	conn, exists := cp.registry.Get(connectionID)
	if !exists {
		return nil
	}
	cp.closeConnection(conn)
	cp.handshakeController.Remove(connectionID)
	return nil
}

func (cp *controlPlane) Send(ctx context.Context, peer Peer, envelope protocol.Envelope) error {
	if err := envelope.Validate(); err != nil {
		return NewIPCErrorWithCause(IPCErrorProtocol, domain.ErrInvalidArgument, "envelope validation failed", err)
	}

	if err := ValidateEnvelopePeer(envelope, peer); err != nil {
		return err
	}

	conn, exists := cp.registry.GetByPeer(peer.Key())
	if !exists {
		return NewIPCErrorWithCause(
			IPCErrorTransport,
			domain.ErrRuntimeUnavailable,
			"no active connection for peer",
			nil,
		)
	}

	if !conn.IsActive() {
		return NewIPCErrorWithCause(
			IPCErrorTransport,
			domain.ErrRuntimeUnavailable,
			"connection is not active",
			nil,
		)
	}

	FillRouting(&envelope, peer)

	if err := conn.Transport().Send(ctx, envelope); err != nil {
		return NewIPCErrorWithCause(IPCErrorTransport, domain.ErrRuntimeUnavailable, "send failed", err)
	}
	return nil
}

func (cp *controlPlane) SendRequest(ctx context.Context, peer Peer, envelope protocol.Envelope, timeout time.Duration) (*protocol.Envelope, error) {
	if err := envelope.Validate(); err != nil {
		return nil, NewIPCErrorWithCause(IPCErrorProtocol, domain.ErrInvalidArgument, "envelope validation failed", err)
	}

	if err := ValidateEnvelopePeer(envelope, peer); err != nil {
		return nil, err
	}

	conn, exists := cp.registry.GetByPeer(peer.Key())
	if !exists {
		return nil, NewIPCErrorWithCause(
			IPCErrorTransport,
			domain.ErrRuntimeUnavailable,
			"no active connection for peer",
			nil,
		)
	}

	if !conn.IsActive() {
		return nil, NewIPCErrorWithCause(
			IPCErrorTransport,
			domain.ErrRuntimeUnavailable,
			"connection is not active",
			nil,
		)
	}

	if envelope.ID == "" {
		return nil, NewIPCError(IPCErrorProtocol, domain.ErrInvalidArgument, "envelope ID is required for request/response")
	}

	if cp.responseCorrelator == nil {
		return nil, NewIPCError(IPCErrorProtocol, domain.ErrInternal, "response correlator not configured")
	}

	respChan, cancel, ok := cp.responseCorrelator.RegisterPending(peer, envelope.ID)
	if !ok {
		return nil, NewIPCError(IPCErrorProtocol, domain.ErrInvalidState, "duplicate or rejected request id")
	}
	defer cancel()

	FillRouting(&envelope, peer)

	if err := conn.Transport().Send(ctx, envelope); err != nil {
		return nil, NewIPCErrorWithCause(IPCErrorTransport, domain.ErrRuntimeUnavailable, "send failed", err)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-respChan:
		if resp == nil {
			return nil, NewIPCError(IPCErrorTransport, domain.ErrRuntimeUnavailable, "connection closed before response received")
		}
		return resp, nil
	case <-timer.C:
		return nil, NewIPCError(IPCErrorTimeout, domain.ErrTimeout, "request timed out")
	case <-ctx.Done():
		return nil, NewIPCError(IPCErrorCancelled, domain.ErrCancelled, "request cancelled")
	}
}

func (cp *controlPlane) Shutdown(ctx context.Context) error {
	cp.mu.Lock()
	if cp.shuttingDown {
		cp.mu.Unlock()
		return nil
	}
	cp.shuttingDown = true
	connections := make([]*Connection, len(cp.connections))
	copy(connections, cp.connections)
	cp.mu.Unlock()

	cp.controlCancel()

	done := make(chan struct{})
	go func() {
		for _, conn := range connections {
			cp.closeConnection(conn)
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cp *controlPlane) receiveLoop(conn *Connection, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			cp.cleanupConnection(conn)
			return
		default:
		}

		envelope, err := conn.Transport().Receive(ctx)
		if err != nil {
			if isTerminalError(err) {
				cp.connHandler.OnDetach(conn)
				cp.cleanupConnection(conn)
				return
			}
			cp.connHandler.OnError(conn, err)
			cp.connHandler.OnDetach(conn)
			cp.cleanupConnection(conn)
			return
		}

		if err := envelope.Validate(); err != nil {
			cp.connHandler.OnError(conn, NewIPCErrorWithCause(
				IPCErrorProtocol,
				domain.ErrProtocolMismatch,
				"envelope validation failed",
				err,
			))
			continue
		}

		if err := ValidateEnvelopePeer(envelope, conn.Peer); err != nil {
			cp.connHandler.OnError(conn, err)
			continue
		}

		cp.processEnvelope(conn, envelope)
	}
}

// processEnvelope applies the handshake gate and routes the envelope accordingly.
//
//   - HandshakeMethod: handled inline; the response is sent back over the transport
//     without invoking the business handlers.
//   - Other methods: blocked via OnError until CanProcess allows them; once allowed,
//     forwarded to the business handler via OnEnvelope.
func (cp *controlPlane) processEnvelope(conn *Connection, envelope protocol.Envelope) {
	if envelope.Method == HandshakeMethod {
		cp.handleHandshakeHello(conn, envelope)
		return
	}

	if cp.responseCorrelator != nil && envelope.Type == protocol.MessageTypeResponse {
		if cp.responseCorrelator.HandleResponse(conn.Peer, &envelope) {
			return
		}
	}

	if !cp.handshakeController.CanProcess(conn.ID, envelope.Method) {
		cp.connHandler.OnError(conn, NewIPCError(
			IPCErrorProtocol,
			domain.ErrInvalidState,
			"handshake required before processing method: "+envelope.Method,
		))
		return
	}

	cp.connHandler.OnEnvelope(conn, envelope)
}

func (cp *controlPlane) handleHandshakeHello(conn *Connection, envelope protocol.Envelope) {
	respPayload, err := cp.handshakeController.HandleHello(
		context.Background(),
		conn.ID,
		conn.Peer,
		envelope.Payload,
	)
	if err != nil {
		cp.sendHelloError(conn, envelope, err)
		cp.connHandler.OnError(conn, err)
		return
	}

	resp := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        envelope.ID,
		RequestID: envelope.ID,
		Payload:   respPayload,
	}
	FillRouting(&resp, conn.Peer)

	if err := conn.Transport().Send(context.Background(), resp); err != nil {
		cp.connHandler.OnError(conn, err)
	}
}

func (cp *controlPlane) sendHelloError(conn *Connection, envelope protocol.Envelope, err error) {
	var perr *protocol.ProtocolError
	if cp.handshakeController != nil {
		perr = cp.handshakeController.MapError(err)
	}
	if perr == nil {
		perr = &protocol.ProtocolError{
			Code:    protocol.ErrorCode(domain.ErrInvalidState),
			Message: err.Error(),
		}
	}
	resp := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeResponse,
		ID:        envelope.ID,
		RequestID: envelope.ID,
		Error:     perr,
	}
	FillRouting(&resp, conn.Peer)
	_ = conn.Transport().Send(context.Background(), resp)
}

func (cp *controlPlane) closeConnection(conn *Connection) {
	if !conn.markClosing(time.Now().UTC()) {
		return
	}

	conn.cancel()

	if cp.responseCorrelator != nil {
		cp.responseCorrelator.CancelByPeer(conn.Peer)
	}

	transport := conn.Transport()
	if transport != nil {
		_ = transport.Close()
	}

	now := time.Now().UTC()
	conn.markClosed(now)
}

func (cp *controlPlane) cleanupConnection(conn *Connection) {
	cp.registry.Remove(conn.ID)
	cp.removeFromConnections(conn.ID)
	cp.handshakeController.Remove(conn.ID)
}

func (cp *controlPlane) removeFromConnections(id ConnectionID) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	for i, c := range cp.connections {
		if c.ID == id {
			cp.connections = append(cp.connections[:i], cp.connections[i+1:]...)
			return
		}
	}
}

func isTerminalError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

const defaultMaxEnvelopeSize = 16 * 1024 * 1024

func GetMaxEnvelopeSize(cp ControlPlane) int64 {
	if c, ok := cp.(*controlPlane); ok {
		return c.maxEnvelopeSize
	}
	return defaultMaxEnvelopeSize
}
