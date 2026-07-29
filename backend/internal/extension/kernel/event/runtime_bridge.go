package event

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type RuntimeDeliveryCallback func(ctx context.Context, delivery Delivery, envelope EventEnvelope, sub *ResolvedSubscription) error

type RuntimeBridge struct {
	mu               sync.RWMutex
	service          *Service
	extensionTypes   map[string][]EventTypeID
	deliveryCallback RuntimeDeliveryCallback
}

func NewRuntimeBridge(service *Service) *RuntimeBridge {
	return &RuntimeBridge{
		service:        service,
		extensionTypes: make(map[string][]EventTypeID),
	}
}

func (b *RuntimeBridge) SetDeliveryCallback(cb RuntimeDeliveryCallback) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deliveryCallback = cb
}

func (b *RuntimeBridge) Attach() {
	b.service.SetDeliveryHandler(b.HandleDelivery)
}

func (b *RuntimeBridge) PublishFromRuntime(ctx context.Context, extensionID string, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	if extensionID == "" {
		return PublishResult{}, fmt.Errorf("event: extension id required")
	}
	if typeID.IsReservedNamespace() {
		return PublishResult{}, fmt.Errorf("%w: extension %s cannot publish reserved namespace %s", ErrNamespaceDenied, extensionID, typeID)
	}
	if !typeID.IsExtensionNamespace(extensionID) {
		return PublishResult{}, fmt.Errorf("%w: extension %s can only publish own namespace, got %s", ErrNamespaceDenied, extensionID, typeID)
	}
	opts.ProducerID = extensionID
	opts.ProducerType = "extension"
	opts.ProducerExtensionID = extensionID
	result, err := b.service.Publish(ctx, typeID, version, payload, opts)
	if err != nil {
		return PublishResult{}, fmt.Errorf("event: publish from runtime: %w", err)
	}
	return result, nil
}

func (b *RuntimeBridge) SubscribeFromRuntime(ctx context.Context, def EventSubscriptionDefinition) error {
	if def.ExtensionID == "" {
		return fmt.Errorf("event: extension id required")
	}
	if def.ContributionID == "" {
		return fmt.Errorf("event: contribution id required")
	}
	if def.Entry == "" {
		return fmt.Errorf("event: entry required")
	}
	def.Enabled = true
	if def.Generation == 0 {
		def.Generation = time.Now().UnixMilli()
	}
	if err := b.service.RegisterSubscription(ctx, def); err != nil {
		return fmt.Errorf("event: subscribe from runtime: %w", err)
	}
	return nil
}

func (b *RuntimeBridge) UnsubscribeFromRuntime(ctx context.Context, extensionID, contributionID string) error {
	sub, ok := b.service.GetSubscription(ctx, contributionID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrSubscriptionNotFound, contributionID)
	}
	if sub.Definition.ExtensionID != extensionID {
		return fmt.Errorf("%w: subscription %s does not belong to extension %s", ErrPermissionDenied, contributionID, extensionID)
	}
	if err := b.service.UnregisterSubscription(ctx, contributionID); err != nil {
		return fmt.Errorf("event: unsubscribe from runtime: %w", err)
	}
	return nil
}

func (b *RuntimeBridge) HandleDelivery(ctx context.Context, delivery Delivery, envelope EventEnvelope, sub *ResolvedSubscription) error {
	b.mu.RLock()
	cb := b.deliveryCallback
	b.mu.RUnlock()
	if cb == nil {
		return fmt.Errorf("%w: delivery callback not registered", ErrRuntimeUnavailable)
	}
	if err := cb(ctx, delivery, envelope, sub); err != nil {
		return fmt.Errorf("event: runtime delivery callback: %w", err)
	}
	return nil
}

func (b *RuntimeBridge) RegisterExtensionEvents(ctx context.Context, extensionID string, generation int64, eventTypes []EventTypeDefinition, subscriptions []EventSubscriptionDefinition) error {
	if extensionID == "" {
		return fmt.Errorf("event: extension id required")
	}
	for i := range eventTypes {
		def := eventTypes[i]
		if def.EventTypeID.IsReservedNamespace() {
			return fmt.Errorf("%w: extension %s cannot register reserved namespace %s", ErrNamespaceDenied, extensionID, def.EventTypeID)
		}
		if !def.EventTypeID.IsExtensionNamespace(extensionID) {
			return fmt.Errorf("%w: extension %s can only register own namespace, got %s", ErrNamespaceDenied, extensionID, def.EventTypeID)
		}
		if err := b.service.RegisterEventType(ctx, def); err != nil {
			return fmt.Errorf("event: register event type %s: %w", def.EventTypeID, err)
		}
	}
	for i := range subscriptions {
		subscriptions[i].ExtensionID = extensionID
		subscriptions[i].Generation = generation
		subscriptions[i].Enabled = true
	}
	if err := b.service.UpdateExtensionGeneration(ctx, extensionID, generation, subscriptions); err != nil {
		return fmt.Errorf("event: update extension generation: %w", err)
	}
	b.mu.Lock()
	registeredTypes := make([]EventTypeID, 0, len(eventTypes))
	for _, t := range eventTypes {
		registeredTypes = append(registeredTypes, t.EventTypeID)
	}
	b.extensionTypes[extensionID] = registeredTypes
	b.mu.Unlock()
	return nil
}

func (b *RuntimeBridge) UnregisterExtensionEvents(ctx context.Context, extensionID string) error {
	if extensionID == "" {
		return fmt.Errorf("event: extension id required")
	}
	if _, err := b.service.CancelDeliveriesByExtension(ctx, extensionID, "extension_uninstalled"); err != nil {
		return fmt.Errorf("event: cancel deliveries for %s: %w", extensionID, err)
	}
	if err := b.service.RemoveSubscriptionsByExtension(ctx, extensionID); err != nil {
		return fmt.Errorf("event: remove subscriptions for %s: %w", extensionID, err)
	}
	b.mu.Lock()
	delete(b.extensionTypes, extensionID)
	b.mu.Unlock()
	return nil
}

func (b *RuntimeBridge) GetExtensionEventTypes(extensionID string) []EventTypeID {
	b.mu.RLock()
	defer b.mu.RUnlock()
	types := b.extensionTypes[extensionID]
	result := make([]EventTypeID, len(types))
	copy(result, types)
	return result
}

type HostEventEmitter interface {
	Emit(ctx context.Context, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitMessageCreated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitMessageSent(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitConversationCreated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitCharacterUpdated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitToolInvocationCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitWorkflowCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitMCPConnectionChanged(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitExtensionEnabled(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitExtensionDisabled(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
	EmitTaskCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error)
}

type hostEventEmitter struct {
	service *Service
}

func NewHostEventEmitter(service *Service) HostEventEmitter {
	return &hostEventEmitter{service: service}
}

func (e *hostEventEmitter) Emit(ctx context.Context, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	if opts.ProducerID == "" {
		opts.ProducerID = "host"
	}
	if opts.ProducerType == "" {
		opts.ProducerType = "host"
	}
	result, err := e.service.Publish(ctx, typeID, version, payload, opts)
	if err != nil {
		return PublishResult{}, fmt.Errorf("event: host emit: %w", err)
	}
	return result, nil
}

func (e *hostEventEmitter) EmitMessageCreated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "message.created", 1, payload, opts)
}

func (e *hostEventEmitter) EmitMessageSent(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "message.sent", 1, payload, opts)
}

func (e *hostEventEmitter) EmitConversationCreated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "conversation.created", 1, payload, opts)
}

func (e *hostEventEmitter) EmitCharacterUpdated(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "character.updated", 1, payload, opts)
}

func (e *hostEventEmitter) EmitToolInvocationCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "tool.invocation_completed", 1, payload, opts)
}

func (e *hostEventEmitter) EmitWorkflowCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "workflow.completed", 1, payload, opts)
}

func (e *hostEventEmitter) EmitMCPConnectionChanged(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "mcp.connection_changed", 1, payload, opts)
}

func (e *hostEventEmitter) EmitExtensionEnabled(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "extension.enabled", 1, payload, opts)
}

func (e *hostEventEmitter) EmitExtensionDisabled(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "extension.disabled", 1, payload, opts)
}

func (e *hostEventEmitter) EmitTaskCompleted(ctx context.Context, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return e.Emit(ctx, "task.completed", 1, payload, opts)
}
