package sdk

import (
	"context"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type Transport interface {
	Send(ctx context.Context, message protocol.Envelope) error
	Receive(ctx context.Context) (protocol.Envelope, error)
	Close() error
}
