package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type EventTypeProvider func(eventTypeID string) (string, int)

type KernelEventAdapter struct {
	publisher      *event.EventPublisher
	typeResolver   EventTypeProvider
	defaultTypeID  string
	defaultVersion int
}

func NewKernelEventAdapter(publisher *event.EventPublisher, defaultType string, defaultVersion int) *KernelEventAdapter {
	return &KernelEventAdapter{
		publisher:      publisher,
		defaultTypeID:  defaultType,
		defaultVersion: defaultVersion,
	}
}

func (a *KernelEventAdapter) WithTypeResolver(fn EventTypeProvider) *KernelEventAdapter {
	if fn != nil {
		a.typeResolver = fn
	}
	return a
}

func (a *KernelEventAdapter) PublishEvent(ctx context.Context, ev EventEnvelope, opts ...PublishEventOption) error {
	if a.publisher == nil {
		return domain.NewHostError(ErrEventFailure, "stream: kernel event publisher is nil")
	}

	cfg := &publishEventConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if err := validateEventEnvelope(ev); err != nil {
		return err
	}

	typeID := a.defaultTypeID
	version := a.defaultVersion
	if a.typeResolver != nil {
		if t, v := a.typeResolver(ev.TypeID); t != "" {
			typeID = t
			version = v
		}
	}
	if ev.TypeID != "" {
		typeID = ev.TypeID
	}

	optsPayload := make(map[string]json.RawMessage)
	for k, v := range ev.Metadata {
		optsPayload[k] = v
	}
	if ev.Method != "" {
		optsPayload["gamehost.method"] = json.RawMessage(fmt.Sprintf("%q", ev.Method))
	}
	if ev.PluginID != "" {
		optsPayload["gamehost.pluginId"] = json.RawMessage(fmt.Sprintf("%q", ev.PluginID))
	}
	if ev.RuntimeID != "" {
		optsPayload["gamehost.runtimeId"] = json.RawMessage(fmt.Sprintf("%q", ev.RuntimeID))
	}
	if ev.ServiceID != "" {
		optsPayload["gamehost.serviceId"] = json.RawMessage(fmt.Sprintf("%q", ev.ServiceID))
	}
	optsPayload["gamehost.eventId"] = json.RawMessage(fmt.Sprintf("%q", ev.ID))
	if ev.OccurredAt > 0 {
		optsPayload["gamehost.occurredAt"] = json.RawMessage(fmt.Sprintf("%d", ev.OccurredAt))
	}

	metadataBytes, _ := json.Marshal(optsPayload)

	var parentEventID string
	var parentDepth int
	if cfg.parentEventID != "" {
		parentEventID = cfg.parentEventID
		parentDepth = cfg.parentDepth
	}

	_, err := a.publisher.Publish(ctx, event.EventTypeID(typeID), version, ev.Payload, event.PublishOptions{
		ProducerID:          ev.PluginID.StringNonEmpty(),
		ProducerType:        pickProducerType(cfg.producerType, ev.PluginID),
		ProducerExtensionID: ev.PluginID.StringNonEmpty(),
		TraceID:             pickTraceID(ev.TraceID, ev.ID),
		OperationID:         ev.Method,
		PartitionKey:        pickPartitionKey(cfg.partitionKey, ev.PluginID, ev.RuntimeID),
		AggregateType:       cfg.aggregateType,
		AggregateID:         cfg.aggregateID,
		ParentEventID:       parentEventID,
		ParentDepth:         parentDepth,
		Metadata:            metadataBytes,
	})

	if err != nil {
		return domain.NewHostError(ErrEventFailure, fmt.Sprintf("stream: kernel event publish failed: %v", err))
	}
	return nil
}

type compositeEventPublisher struct {
	primary    EventPublisher
	secondaries []EventPublisher
}

func NewCompositeEventPublisher(primary EventPublisher, others ...EventPublisher) EventPublisher {
	return &compositeEventPublisher{primary: primary, secondaries: others}
}

func (c *compositeEventPublisher) PublishEvent(ctx context.Context, event EventEnvelope, opts ...PublishEventOption) error {
	if c.primary != nil {
		if err := c.primary.PublishEvent(ctx, event, opts...); err != nil {
			return err
		}
	}
	for _, sec := range c.secondaries {
		if sec == nil {
			continue
		}
		_ = sec.PublishEvent(ctx, event, opts...)
	}
	return nil
}

func validateEventEnvelope(ev EventEnvelope) error {
	if ev.PluginID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream event: plugin id must not be empty")
	}
	if ev.RuntimeID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream event: runtime id must not be empty")
	}
	if ev.ServiceID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream event: service id must not be empty")
	}
	if ev.TypeID == "" && ev.Method == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "stream event: either type id or method must be set")
	}
	return nil
}

func pickProducerType(cfgType string, pluginID domain.PluginID) string {
	if cfgType != "" {
		return cfgType
	}
	if pluginID == "" {
		return "host"
	}
	return "gamehost_plugin"
}

func pickTraceID(traceID, eventID string) string {
	if traceID != "" {
		return traceID
	}
	return eventID
}

func pickPartitionKey(cfgKey string, pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID) string {
	if cfgKey != "" {
		return cfgKey
	}
	if runtimeID != "" {
		return fmt.Sprintf("%s/%s", pluginID, runtimeID)
	}
	return string(pluginID)
}

func buildMetadata(metadata map[string]json.RawMessage) json.RawMessage {
	if len(metadata) == 0 {
		return nil
	}
	b, _ := json.Marshal(metadata)
	return b
}

var _ = time.Now
