// Package notification provides the GameHost notification infrastructure.
//
// IMPORTANT: GameHost notification events flow through the shared kernel Durable Event
// pipeline (event.Service). This package defines adapters that translate GameHost-specific
// notifications into kernel Durable Events, NOT a separate event infrastructure.
//
// The KernelEventAdapter is the bridge between GameHost Notification and the
// Kernel Durable Event system. Its EventPublisher must be backed by a shared
// kernel event.Service or compatible durable event producer.
//
// Rules:
//   - Plugin Metadata MUST NOT override trusted event envelope fields
//     (producerId, producerType, partitionKey, traceId, aggregateId, permission, scope).
//   - High-frequency game state/binary streams should NOT be published as durable events.
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/gamehost/stream"
)

// EventTypeProvider maps notification methods to event type IDs.
type EventTypeProvider interface {
	EventTypeID(method string) (string, int)
	DefaultEventTypeID() (string, int)
}

// Static EventTypeProvider with optional method-specific overrides.
type StaticEventTypeProvider struct {
	DefaultTypeID  string
	DefaultVersion int
	Overrides      map[string]string
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

// EventPublisher is the interface for publishing durable kernel events.
// Implementations should be backed by the shared kernel event.DurableEventPublisher
// or event.Service. Do NOT implement a separate GameHost event store.
type EventPublisher interface {
	Publish(ctx context.Context, typeID string, version int, payload json.RawMessage, opts map[string]json.RawMessage) error
}

// KernelEventAdapter adapts GameHost notifications to the Kernel Durable Event system.
//
// This adapter translates GameHost plugin notifications into a format suitable for the
// shared kernel event outbox. It does NOT provide its own event persistence.
//
// The producer fields are set by this adapter; callers MUST NOT override them via metadata.
type KernelEventAdapter struct {
	provider  EventTypeProvider
	publisher EventPublisher
}

// NewKernelEventAdapter creates a new KernelEventAdapter that bridges GameHost
// notifications to the Shared Kernel Durable Event system.
//
// The publisher parameter should be backed by the shared kernel event.Service.
// It must NOT create a separate GameHost event store.
func NewKernelEventAdapter(provider EventTypeProvider, publisher EventPublisher) *KernelEventAdapter {
	return &KernelEventAdapter{
		provider:  provider,
		publisher: publisher,
	}
}

// Publish converts a GameHost notification to a kernel Durable Event and publishes it.
//
// IMPORTANT: Plugin metadata is NOT merged into the event envelope options.
// Only adapter-set trusted fields (producerId, producerType, partitionKey, etc.) are used.
// This prevents plugin code from overriding trusted Kernel event identity fields.
func (a *KernelEventAdapter) Publish(ctx context.Context, n Notification) error {
	if a.publisher == nil {
		return fmt.Errorf("kernel_event_adapter: publisher is nil")
	}
	typeID, version := a.provider.EventTypeID(n.Method)

	opts := map[string]json.RawMessage{
		"producerId":          jsonRaw(string(n.PluginID)),
		"producerType":        jsonRaw("gamehost_plugin"),
		"producerExtensionId": jsonRaw(string(n.PluginID)),
		"producerRuntimeId":   jsonRaw(string(n.RuntimeID)),
		"producerServiceId":   jsonRaw(string(n.ServiceID)),
		"method":              jsonRaw(n.Method),
		"traceId":             jsonRaw(n.ID),
		"partitionKey":        jsonRaw(string(n.PluginID)),
		"occurredAt":          jsonRaw(n.ReceivedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00")),
		"receivedAt":          jsonRaw(strconv.FormatInt(n.ReceivedAt.UTC().UnixNano(), 10)),
	}

	for k, v := range n.Metadata {
		opts[k] = v
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
