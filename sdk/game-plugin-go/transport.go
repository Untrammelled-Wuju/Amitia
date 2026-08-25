package sdk

import (
	"context"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type Transport interface {
	Send(ctx context.Context, message protocol.Envelope) error
	Receive(ctx context.Context) (protocol.Envelope, error)
	Close() error
}
