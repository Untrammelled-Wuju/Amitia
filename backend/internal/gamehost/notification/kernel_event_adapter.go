package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/gamehost/stream"
)

type EventTypeProvider interface {
	EventTypeID(method string) (string, int)
	DefaultEventTypeID() (string, int)
}

type StaticEventTypeProvider struct {
	DefaultTypeID   string
	DefaultVersion  int
	Overrides       map[string]string
}

func NewStaticEventTypeProvider(typeID string, version int) *StaticEventTypeProvider {
	return &StaticEventTypeProvider{
		DefaultTypeID:  typeID,
		DefaultVersion: version,
		Overrides:      make(map[string]string),
	}
}

func (p *StaticEventTypeProvider) WithOverride(method, typeID string) *StaticEventTypeProvider {
	p.Overrides[method] = typeID
	return p
}

func (p *StaticEventTypeProvider) EventTypeID(method string) (string, int) {
	if t, ok := p.Overrides[method]; ok {
		return t, p.DefaultVersion
	}
	return p.DefaultTypeID, p.DefaultVersion
}

func (p *StaticEventTypeProvider) DefaultEventTypeID() (string, int) {
	return p.DefaultTypeID, p.DefaultVersion
}

type EventPublisher interface {
	Publish(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error
}

type KernelEventAdapter struct {
	provider EventTypeProvider
	publisher EventPublisher
}

func NewKernelEventAdapter(provider EventTypeProvider, publisher EventPublisher) *KernelEventAdapter {
	return &KernelEventAdapter{
		provider:  provider,
		publisher: publisher,
	}
}

func (a *KernelEventAdapter) Publish(ctx context.Context, n Notification) error {
	if a.publisher == nil {
		return fmt.Errorf("kernel_event_adapter: publisher is nil")
	}
	typeID, version := a.provider.EventTypeID(n.Method)

	opts := map[string]json.RawMessage{
		"producerId":    jsonRaw(string(n.PluginID)),
		"producerType":  jsonRaw("gamehost_plugin"),
		"producerExtensionId": jsonRaw(string(n.PluginID)),
		"producerRuntimeId": jsonRaw(string(n.RuntimeID)),
		"producerServiceId": jsonRaw(string(n.ServiceID)),
		"method":        jsonRaw(n.Method),
		"traceId":       jsonRaw(n.ID),
		"partitionKey":  jsonRaw(string(n.PluginID)),
		"occurredAt":    jsonRaw(n.ReceivedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00")),
		"receivedAt":    jsonRaw(strconv.FormatInt(n.ReceivedAt.UTC().UnixNano(), 10)),
	}

	if len(opts) > 0 && n.Metadata != nil {
		for k, v := range n.Metadata {
			opts[k] = v
		}
	}

	return a.publisher.Publish(ctx, typeID, version, n.Payload, opts)
}

func jsonRaw(s string) json.RawMessage {
	return json.RawMessage(`"` + s + `"`)
}

func BuildGameHostProvider() *StaticEventTypeProvider {
	return NewStaticEventTypeProvider("gamehost.notification", 1)
}

type streamEventPublisherBridge struct {
	adapter *KernelEventAdapter
}

func (s *streamEventPublisherBridge) PublishEvent(ctx context.Context, ev stream.EventEnvelope, opts ...stream.PublishEventOption) error {
	n := notificationFromEventEnvelope(ev)
	return s.adapter.Publish(ctx, n)
}

func (a *KernelEventAdapter) AsStreamPublisher() stream.EventPublisher {
	return &streamEventPublisherBridge{adapter: a}
}

func notificationFromEventEnvelope(ev stream.EventEnvelope) Notification {
	var receivedAt time.Time
	if ev.OccurredAt > 0 {
		receivedAt = time.Unix(0, ev.OccurredAt)
	} else {
		receivedAt = time.Now().UTC()
	}
	return Notification{
		ID:         ev.ID,
		PluginID:   ev.PluginID,
		RuntimeID:  ev.RuntimeID,
		ServiceID:  ev.ServiceID,
		Method:     ev.Method,
		Payload:    ev.Payload,
		Metadata:   ev.Metadata,
		ReceivedAt: receivedAt,
	}
}
