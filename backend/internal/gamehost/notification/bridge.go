package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type NotificationBridge interface {
	Handle(
		ctx context.Context,
		source RouteContext,
		method string,
		payload json.RawMessage,
		metadata map[string]json.RawMessage,
	) error
}

type Bridge struct {
	sink    NotificationSink
	nowFunc func() time.Time
}

func NewBridge(sink NotificationSink) *Bridge {
	return &Bridge{
		sink:    sink,
		nowFunc: func() time.Time { return time.Now().UTC() },
	}
}

func (b *Bridge) Handle(
	ctx context.Context,
	source RouteContext,
	method string,
	payload json.RawMessage,
	metadata map[string]json.RawMessage,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateRoute(source); err != nil {
		return err
	}
	if err := ValidateMethod(method); err != nil {
		return err
	}
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

	notification := Notification{
		PluginID:   source.PluginID,
		RuntimeID:  source.RuntimeID,
		ServiceID:  source.ServiceID,
		Generation: source.Generation,
		Method:     method,
		Payload:    deepCopyRaw(payload),
		Metadata:   deepCopyMetadata(metadata),
		ReceivedAt: b.nowFunc(),
	}

	if b.sink == nil {
		return errors.New("notification: sink is nil")
	}
	return b.sink.Publish(ctx, notification)
}

func (b *Bridge) Sink() NotificationSink {
	return b.sink
}

func (b *Bridge) SetNowFunc(f func() time.Time) {
	if f != nil {
		b.nowFunc = f
	}
}
