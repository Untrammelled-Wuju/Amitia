package sdk

import (
	"context"

	"github.com/u-ai/game-plugin-sdk-go/protocol"
)

type RequestHandler func(ctx context.Context, request protocol.Envelope) (any, error)

type NotificationHandler func(ctx context.Context, notification protocol.Envelope) error
