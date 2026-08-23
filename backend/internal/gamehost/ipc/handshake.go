package ipc

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

// HandshakeMethod is the well-known RPC method name that initiates protocol negotiation.
// Connections MUST complete a handshake before any business traffic is allowed.
const HandshakeMethod = "control.handshake.hello"

// HandshakeController is the boundary through which the IPC layer interacts with the
// protocol handshake sub-system without importing the handshake package directly.
//
// Implementations are responsible for:
//   - Tracking per-connection handshake state
//   - Validating / executing hello requests
//   - Reporting whether a given connection may process a specific method
//
// The IPC layer guarantees that Register / Remove are paired per connection lifecycle,
// and that method-level gating (CanProcess) is consulted before every business envelope.
type HandshakeController interface {
	// Register marks a connection as in-progress for handshake.
	Register(id ConnectionID)

	// Remove clears all handshake state for a connection.
	Remove(id ConnectionID)

	// HandleHello processes a hello request and returns the JSON-encodable response
	// payload on success. On failure it returns a non-nil error describing the reason.
	HandleHello(
		ctx context.Context,
		connID ConnectionID,
		peer Peer,
		payload json.RawMessage,
	) (json.RawMessage, error)

	// ConfirmReady commits the ready state only after the hello response has
	// been successfully written to the transport.
	ConfirmReady(id ConnectionID)

	// CanProcess reports whether the given connection is allowed to process the
	// specified method name right now. Implementations MUST return true for
	// HandshakeMethod (and any other pre-ready allowlisted methods) regardless of
	// ready state; they MUST return false for any business method until the connection
	// has completed a successful handshake.
	CanProcess(connID ConnectionID, method string) bool

	// MapError translates a handshake failure into a protocol error suitable
	// for inclusion in an Envelope.Error field. It MUST return nil only when
	// the input error is itself nil; for any non-nil input it MUST return a
	// non-nil *protocol.ProtocolError.
	MapError(err error) *protocol.ProtocolError
}

// NoopHandshakeController is the default HandshakeController used when the application
// does not require protocol negotiation. It allows all traffic unconditionally.
type NoopHandshakeController struct{}

func (n *NoopHandshakeController) Register(_ ConnectionID) {}

func (n *NoopHandshakeController) Remove(_ ConnectionID) {}

func (n *NoopHandshakeController) HandleHello(
	_ context.Context,
	_ ConnectionID,
	_ Peer,
	_ json.RawMessage,
) (json.RawMessage, error) {
	return nil, NewIPCError(
		IPCErrorProtocol,
		domain.ErrProtocolMismatch,
		"handshake is not enabled",
	)
}

func (n *NoopHandshakeController) ConfirmReady(_ ConnectionID) {}

func (n *NoopHandshakeController) CanProcess(_ ConnectionID, _ string) bool {
	return true
}

func (n *NoopHandshakeController) MapError(err error) *protocol.ProtocolError {
	if err == nil {
		return nil
	}
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode(domain.ErrInvalidState),
		Message: err.Error(),
	}
}

// NewNoopHandshakeController returns the shared no-op controller instance.
func NewNoopHandshakeController() *NoopHandshakeController {
	return &NoopHandshakeController{}
}
