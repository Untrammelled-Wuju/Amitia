package transport

import (
	"context"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateClosing  State = "closing"
	StateError    State = "error"
)

type MCPTransport interface {
	Start(context.Context) error
	Send(context.Context, protocol.Message) error
	Receive() <-chan protocol.Message
	Close(context.Context) error
	State() State
}
