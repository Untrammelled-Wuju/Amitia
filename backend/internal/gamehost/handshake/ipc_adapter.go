package handshake

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

// Ensure *HandshakeControllerAdapter satisfies ipc.HandshakeController at compile time.
var _ ipc.HandshakeController = (*HandshakeControllerAdapter)(nil)

// HandshakeControllerAdapter bridges a HandshakeManager to the ipc.HandshakeController
// interface so the IPC layer can drive the protocol handshake without a circular import.
type HandshakeControllerAdapter struct {
	inner *HandshakeManager
}

// NewHandshakeControllerAdapter returns an ipc.HandshakeController backed by the
// supplied HandshakeManager. The manager must be fully configured before the
// returned controller is passed into ipc.ControlPlaneConfig.
func NewHandshakeControllerAdapter(mgr *HandshakeManager) *HandshakeControllerAdapter {
	return &HandshakeControllerAdapter{inner: mgr}
}

// Register registers the connection ID with the underlying manager.
func (a *HandshakeControllerAdapter) Register(id ipc.ConnectionID) {
	a.inner.RegisterConnection(string(id))
}

// Remove clears all handshake state for the given connection.
func (a *HandshakeControllerAdapter) Remove(id ipc.ConnectionID) {
	a.inner.RemoveConnection(string(id))
}

// HandleHello parses the hello envelope payload, delegates to the manager, and
// encodes the successful response as JSON. Any error path returns a non-nil
// domain error; the IPC layer surfaces it to the caller.
func (a *HandshakeControllerAdapter) HandleHello(
	ctx context.Context,
	connID ipc.ConnectionID,
	peer ipc.Peer,
	payload json.RawMessage,
) (json.RawMessage, error) {
	resp, err := a.inner.HandleHelloFromEnvelope(ctx, string(connID), peer, payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

// CanProcess implements the ReadyGate-style method rule:
//   - HandshakeMethod is always allowed (in-progress peers must be able to send it)
//   - any pre-ready allowlisted method is allowed
//   - any other method requires the connection to be in HandshakeStateReady
func (a *HandshakeControllerAdapter) CanProcess(connID ipc.ConnectionID, method string) bool {
	if method == ipc.HandshakeMethod {
		return true
	}
	state, ok := a.inner.GetState(string(connID))
	if !ok {
		return false
	}
	return state == HandshakeStateReady
}

// MapError converts a handshake failure into a protocol ProtocolError. It
// returns nil only for a nil input error.
func (a *HandshakeControllerAdapter) MapError(err error) *protocol.ProtocolError {
	if err == nil {
		return nil
	}
	if he, ok := err.(*HandshakeError); ok {
		wrapped := MapToProtocolError(he)
		return &protocol.ProtocolError{
			Code:    protocol.ErrorCode(wrapped.Code),
			Message: wrapped.Message,
		}
	}
	return &protocol.ProtocolError{
		Code:    protocol.ErrorCode(domain.ErrInternal),
		Message: err.Error(),
	}
}
