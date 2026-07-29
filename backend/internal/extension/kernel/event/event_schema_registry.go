package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type EventSchemaRegistry struct {
	mu     sync.RWMutex
	types  map[string]map[int]EventTypeDefinition
	latest map[string]int
}

func NewEventSchemaRegistry() *EventSchemaRegistry {
	return &EventSchemaRegistry{
		types:  make(map[string]map[int]EventTypeDefinition),
		latest: make(map[string]int),
	}
}

func (r *EventSchemaRegistry) RegisterEventType(ctx context.Context, def EventTypeDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}
	if def.DefinitionHash == "" {
		def.DefinitionHash = def.Hash()
	}
	if def.EventTypeID.IsReservedNamespace() {
		if def.ProducerPolicy.RequireSystemTrust && !isSystemProducer(def.ProducerPolicy) {
			return fmt.Errorf("%w: reserved namespace %s requires system trust", ErrNamespaceDenied, def.EventTypeID)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := string(def.EventTypeID)
	versions, ok := r.types[key]
	if !ok {
		versions = make(map[int]EventTypeDefinition)
		r.types[key] = versions
	}
	if existing, ok := versions[def.Version]; ok {
		if existing.DefinitionHash != def.DefinitionHash {
			return fmt.Errorf("%w: %s v%d hash mismatch", ErrSchemaConflict, def.EventTypeID, def.Version)
		}
		return nil
	}
	versions[def.Version] = def
	if def.Version > r.latest[key] {
		r.latest[key] = def.Version
	}
	return nil
}

func (r *EventSchemaRegistry) GetEventType(ctx context.Context, typeID EventTypeID, version int) (EventTypeDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.types[string(typeID)]
	if !ok {
		return EventTypeDefinition{}, fmt.Errorf("%w: %s", ErrSchemaNotFound, typeID)
	}
	if version <= 0 {
		version = r.latest[string(typeID)]
		if version == 0 {
			return EventTypeDefinition{}, fmt.Errorf("%w: %s no versions", ErrSchemaNotFound, typeID)
		}
	}
	def, ok := versions[version]
	if !ok {
		return EventTypeDefinition{}, fmt.Errorf("%w: %s v%d", ErrSchemaNotFound, typeID, version)
	}
	return def, nil
}

func (r *EventSchemaRegistry) ListEventTypes(ctx context.Context) ([]EventTypeDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []EventTypeDefinition
	for _, versions := range r.types {
		for _, def := range versions {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].EventTypeID != result[j].EventTypeID {
			return result[i].EventTypeID < result[j].EventTypeID
		}
		return result[i].Version < result[j].Version
	})
	return result, nil
}

func (r *EventSchemaRegistry) ListByNamespace(ctx context.Context, namespace string) ([]EventTypeDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []EventTypeDefinition
	for key, versions := range r.types {
		if namespaceMatch(key, namespace) {
			for _, def := range versions {
				result = append(result, def)
			}
		}
	}
	return result, nil
}

func (r *EventSchemaRegistry) IsRegistered(ctx context.Context, typeID EventTypeID, version int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions, ok := r.types[string(typeID)]
	if !ok {
		return false
	}
	if version <= 0 {
		return len(versions) > 0
	}
	_, ok = versions[version]
	return ok
}

func (r *EventSchemaRegistry) LatestVersion(typeID EventTypeID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.latest[string(typeID)]
}

func (r *EventSchemaRegistry) ValidatePayload(ctx context.Context, typeID EventTypeID, version int, payload []byte) error {
	def, err := r.GetEventType(ctx, typeID, version)
	if err != nil {
		return err
	}
	if int64(len(payload)) > def.MaxPayloadBytes {
		return fmt.Errorf("%w: payload %d exceeds %d", ErrInvalidPayload, len(payload), def.MaxPayloadBytes)
	}
	if def.PayloadSchema == nil || len(def.PayloadSchema) == 0 {
		return nil
	}
	return validateJSONSchema(payload, def.PayloadSchema)
}

func isSystemProducer(policy EventProducerPolicy) bool {
	for _, p := range policy.AllowedProducers {
		if p == "host" || p == "system" {
			return true
		}
	}
	return len(policy.AllowedProducers) == 0
}

func namespaceMatch(typeID, namespace string) bool {
	if namespace == "" {
		return true
	}
	if len(namespace) > 0 && namespace[len(namespace)-1] == '.' {
		return startsWith(string(typeID), namespace)
	}
	return string(typeID) == namespace || startsWith(string(typeID), namespace+".")
}

func startsWith(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func validateJSONSchema(payload, schema []byte) error {
	if len(schema) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return fmt.Errorf("%w: payload is not json object: %v", ErrInvalidPayload, err)
	}
	var s struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil
	}
	for _, req := range s.Required {
		if _, ok := raw[req]; !ok {
			return fmt.Errorf("%w: missing required field %s", ErrInvalidPayload, req)
		}
	}
	return nil
}

func DefaultHostEventTypes() []EventTypeDefinition {
	maxPayload := int64(256 * 1024)
	maxMeta := int64(32 * 1024)
	return []EventTypeDefinition{
		{
			EventTypeID: "message.created", Version: 1,
			Description:     "Message created in conversation",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelMedium,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 64, RequireApproval: true, RequiredPermissions: []string{"event.subscribe", "message.read"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
			RetentionPolicy:  EventRetentionPolicy{MaxAge: 7 * 24 * 60 * 60 * 1e9, MaxDeliveryCount: 5},
			SensitiveFields: []SensitiveFieldRule{
				{Path: "text", Classification: "message_content", DefaultAction: SensitiveAllowWithPermission, RequiredPermission: []PermissionRequirement{{Permission: "message.read"}}},
				{Path: "attachments", Classification: "attachments", DefaultAction: SensitiveAllowWithPermission, RequiredPermission: []PermissionRequirement{{Permission: "message.read"}}},
				{Path: "context", Classification: "context", DefaultAction: SensitiveOmit},
				{Path: "systemPrompt", Classification: "prompt", DefaultAction: SensitiveOmit},
			},
			ProjectionRules: []EventProjectionRule{
				{SourcePath: "messageId", TargetPath: "messageId"},
				{SourcePath: "conversationId", TargetPath: "conversationId"},
				{SourcePath: "characterId", TargetPath: "characterId"},
				{SourcePath: "direction", TargetPath: "direction"},
				{SourcePath: "messageType", TargetPath: "messageType"},
				{SourcePath: "createdAt", TargetPath: "createdAt"},
			},
		},
		{
			EventTypeID: "message.sent", Version: 1,
			Description:     "Message sent to channel",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelMedium,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 64, RequireApproval: true, RequiredPermissions: []string{"event.subscribe", "message.read"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
		{
			EventTypeID: "message.delivery_failed", Version: 1,
			Description:     "Message delivery to channel failed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelHigh,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 3, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
		{
			EventTypeID: "conversation.created", Version: 1,
			Description:     "Conversation created",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 64, RequireApproval: true, RequiredPermissions: []string{"event.subscribe", "conversation.read"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
		{
			EventTypeID: "character.updated", Version: 1,
			Description:     "Character updated",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 64, RequireApproval: true, RequiredPermissions: []string{"event.subscribe", "character.read"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
		{
			EventTypeID: "tool.invocation_completed", Version: 1,
			Description:     "Tool invocation completed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "workflow.completed", Version: 1,
			Description:     "Workflow execution completed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
		{
			EventTypeID: "mcp.connection_changed", Version: 1,
			Description:     "MCP connection state changed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.enabled", Version: 1,
			Description:     "Extension enabled",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.disabled", Version: 1,
			Description:     "Extension disabled",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.installed", Version: 1,
			Description:     "Extension installed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.uninstalled", Version: 1,
			Description:     "Extension uninstalled",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.upgraded", Version: 1,
			Description:     "Extension upgraded to new version",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "extension.runtime_crashed", Version: 1,
			Description:     "Extension runtime crashed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelHigh,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 16, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 3, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 4},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "task.completed", Version: 1,
			Description:     "Task execution completed",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelLow,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: true, MaxSubscribers: 32, RequireApproval: true, RequiredPermissions: []string{"event.subscribe"}},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 5e9, MaxAttempts: 5, InitialBackoff: 1e9, MaxBackoff: 3e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingPerPartition, MaxInFlight: 4},
			OrderingPolicy:   OrderingPerPartition,
		},
	}
}

func InternalOnlyEventTypes() []EventTypeDefinition {
	maxPayload := int64(64 * 1024)
	maxMeta := int64(8 * 1024)
	return []EventTypeDefinition{
		{
			EventTypeID: "permission.raw_decision", Version: 1,
			Description:     "Raw permission decision (internal only)",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelCritical,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: false, MaxSubscribers: 4, RequireApproval: true},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 2e9, MaxAttempts: 3, InitialBackoff: 1e9, MaxBackoff: 1e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 2},
			OrderingPolicy:   OrderingNone,
		},
		{
			EventTypeID: "secret.used", Version: 1,
			Description:     "Secret was used (internal only)",
			MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta,
			RiskLevel:        RiskLevelCritical,
			ProducerPolicy:   EventProducerPolicy{AllowedProducers: []string{"host", "system"}, RequireSystemTrust: true, MaxPayloadBytes: maxPayload, MaxMetadataBytes: maxMeta},
			SubscriberPolicy: EventSubscriberPolicy{AllowThirdParty: false, MaxSubscribers: 2, RequireApproval: true},
			DeliveryPolicy:   EventDeliveryPolicy{Timeout: 2e9, MaxAttempts: 3, InitialBackoff: 1e9, MaxBackoff: 1e8, BackoffMultiplier: 2, JitterFactor: 0.2, OrderingRequirement: OrderingNone, MaxInFlight: 2},
			OrderingPolicy:   OrderingNone,
		},
	}
}

var _ = errors.New
