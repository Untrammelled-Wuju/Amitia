package behavior

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type EventSchemaDef struct {
	EventType     string
	Reliability   EventReliability
	DefaultTTL    time.Duration
	AllowedFields map[string]string
	SchemaVersion int
}

var schemaRegistry = map[string]*EventSchemaDef{}
var registryFrozen bool

func RegisterSchema(def *EventSchemaDef) error {
	if registryFrozen {
		return fmt.Errorf("schema registry is frozen")
	}
	if def == nil {
		return fmt.Errorf("schema def is nil")
	}
	if def.EventType == "" {
		return fmt.Errorf("schema def missing eventType")
	}
	if _, exists := schemaRegistry[def.EventType]; exists {
		return fmt.Errorf("duplicate schema registration: %s", def.EventType)
	}
	if def.SchemaVersion == 0 {
		def.SchemaVersion = 1
	}
	schemaRegistry[def.EventType] = def
	return nil
}

func GetSchema(eventType string) (*EventSchemaDef, bool) {
	def, ok := schemaRegistry[eventType]
	return def, ok
}

func IsSchemaRegistered(eventType string) bool {
	_, ok := schemaRegistry[eventType]
	return ok
}

func ListRegisteredEventTypes() []string {
	types := make([]string, 0, len(schemaRegistry))
	for t := range schemaRegistry {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func ValidatePayload(eventType string, payload json.RawMessage) (json.RawMessage, error) {
	def, ok := schemaRegistry[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}
	filtered := make(map[string]interface{})
	for field, expectedType := range def.AllowedFields {
		if val, exists := raw[field]; exists {
			if !checkType(val, expectedType) {
				return nil, fmt.Errorf("field %s has wrong type, expected %s", field, expectedType)
			}
			filtered[field] = val
		}
	}
	result, _ := json.Marshal(filtered)
	return result, nil
}

func checkType(val interface{}, expected string) bool {
	switch expected {
	case "string":
		_, ok := val.(string)
		return ok
	case "int", "int64":
		switch val.(type) {
		case float64, int, int64, json.Number:
			return true
		}
		return false
	case "float64":
		switch val.(type) {
		case float64, int, json.Number:
			return true
		}
		return false
	case "bool":
		_, ok := val.(bool)
		return ok
	case "any":
		return true
	default:
		return true
	}
}

func ComputeExpiresAt(eventType string, occurredAt time.Time) *time.Time {
	def, ok := schemaRegistry[eventType]
	if !ok {
		return nil
	}
	if def.DefaultTTL <= 0 {
		return nil
	}
	t := occurredAt.Add(def.DefaultTTL)
	return &t
}

func GetReliability(eventType string) EventReliability {
	def, ok := schemaRegistry[eventType]
	if !ok {
		return ReliabilityRecoverable
	}
	return def.Reliability
}

func ComputeSchemaHash(eventType string) string {
	def, ok := schemaRegistry[eventType]
	if !ok {
		return ""
	}
	buf, _ := json.Marshal(def)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func ComputeRegistryHash() string {
	keys := make([]string, 0, len(schemaRegistry))
	for k := range schemaRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		def := schemaRegistry[k]
		buf, _ := json.Marshal(def)
		h.Write(buf)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func FreezeRegistry() {
	registryFrozen = true
}

func init() {
	registerInteractionSchemas()
	registerVoiceSchemas()
	registerToolSchemas()
	registerAffectSchemas()
	registerActivitySchemas()
	registerProactiveSchemas()
	registerDesktopSchemas()
	registerPlaybackSchemas()
	registerRuntimeSchemas()
	registerDeliverySchemas()
	FreezeRegistry()
}

func registerInteractionSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"chat.message.received", ReliabilityRecoverable, 5 * time.Second, map[string]string{
			"messageId": "string", "channel": "string", "contentType": "string",
			"hasMedia": "bool", "interactionStatusVersion": "int64",
		}},
		{"chat.context.loading", ReliabilityRecoverable, 15 * time.Second, map[string]string{
			"interactionId": "string", "statusVersion": "int64", "runtimePathHash": "string",
		}},
		{"chat.response.started", ReliabilityRecoverable, 120 * time.Second, map[string]string{
			"interactionId": "string", "statusVersion": "int64",
			"modelProviderClass": "string", "streaming": "bool",
		}},
		{"chat.response.ready", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"interactionId": "string", "statusVersion": "int64",
			"responseMode": "string", "forceVoice": "bool",
		}},
		{"chat.response.completed", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"interactionId": "string", "commitId": "string",
			"messageCount": "int", "responseGroupId": "string", "origin": "string",
		}},
		{"chat.response.failed", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"interactionId": "string", "statusVersion": "int64",
			"errorClass": "string", "retryable": "bool",
		}},
		{"chat.response.cancelled", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"interactionId": "string", "statusVersion": "int64", "cancelReason": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerDeliverySchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"delivery.started", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"interactionId": "string", "responseGroupId": "string",
			"channel": "string", "deliverySequence": "int64",
		}},
		{"delivery.completed", ReliabilityDurable, 7 * 24 * time.Hour, map[string]string{
			"deliveryId": "string", "responseGroupId": "string",
			"channel": "string", "status": "string",
		}},
		{"delivery.failed", ReliabilityDurable, 7 * 24 * time.Hour, map[string]string{
			"deliveryId": "string", "responseGroupId": "string",
			"channel": "string", "errorClass": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerVoiceSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"voice.session.started", ReliabilityRecoverable, 0, map[string]string{
			"sessionId": "string",
		}},
		{"voice.listening.started", ReliabilityRecoverable, 15 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.listening.activity", ReliabilityEphemeral, 2 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.listening.ended", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.processing.started", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.speaking.started", ReliabilityRecoverable, 0, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.speaking.ended", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.turn.interrupted", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"sessionId": "string", "turnId": "string",
		}},
		{"voice.session.ended", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"sessionId": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerToolSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"agent.tool.started", ReliabilityRecoverable, 5 * time.Minute, map[string]string{
			"toolOperationId": "string", "toolCategory": "string",
			"displayClass": "string", "depth": "int", "expectedLongRunning": "bool",
		}},
		{"agent.tool.progress", ReliabilityEphemeral, 5 * time.Second, map[string]string{
			"toolOperationId": "string",
		}},
		{"agent.tool.completed", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"toolOperationId": "string",
		}},
		{"agent.tool.failed", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"toolOperationId": "string", "errorClass": "string",
		}},
		{"agent.tool.cancelled", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"toolOperationId": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerAffectSchemas() {
	RegisterSchema(&EventSchemaDef{
		EventType:   "character.affect.changed",
		Reliability: ReliabilityDurable,
		DefaultTTL:  24 * time.Hour,
		AllowedFields: map[string]string{
			"version": "string", "valence": "float64", "arousal": "float64",
			"tension": "float64", "stress": "float64",
			"label": "string", "confidence": "float64",
			"prevLabel": "string", "deltaMagnitude": "float64",
		},
		SchemaVersion: 1,
	})
}

func registerActivitySchemas() {
	RegisterSchema(&EventSchemaDef{
		EventType:   "character.activity.changed",
		Reliability: ReliabilityDurable,
		DefaultTTL:  0,
		AllowedFields: map[string]string{
			"activityKey": "string", "source": "string", "confidence": "float64",
			"version": "string",
		},
		SchemaVersion: 1,
	})
	RegisterSchema(&EventSchemaDef{
		EventType:   "character.time_period.changed",
		Reliability: ReliabilityDurable,
		DefaultTTL:  0,
		AllowedFields: map[string]string{
			"timePeriod": "string", "effectiveLocalDate": "string", "timezone": "string",
		},
		SchemaVersion: 1,
	})
}

func registerProactiveSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"proactive.message.started", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"correlationId": "string", "ruleId": "string", "intent": "string",
		}},
		{"proactive.message.completed", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"correlationId": "string", "interactionId": "string",
		}},
		{"proactive.message.suppressed", ReliabilityDurable, 24 * time.Hour, map[string]string{
			"correlationId": "string", "reason": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerDesktopSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"runtime.pointer.clicked", ReliabilityEphemeral, 2 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.pointer.double_clicked", ReliabilityEphemeral, 2 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.pointer.hovered", ReliabilityEphemeral, 1 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.drag.started", ReliabilityEphemeral, 2 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.drag.moved", ReliabilityEphemeral, 500 * time.Millisecond, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.drag.completed", ReliabilityEphemeral, 3 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.drag.cancelled", ReliabilityEphemeral, 3 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.pet.fall.started", ReliabilityEphemeral, 3 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64",
		}},
		{"runtime.pet.edge.reached", ReliabilityEphemeral, 3 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64", "edge": "string",
		}},
		{"runtime.pet.interacted", ReliabilityEphemeral, 5 * time.Second, map[string]string{
			"gestureId": "string", "sequence": "int64", "interactionType": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerPlaybackSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"runtime.playback.action_started", ReliabilityRecoverable, 30 * time.Second, map[string]string{
			"commandId": "string", "decisionId": "string", "actionKey": "string",
		}},
		{"runtime.playback.action_completed", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"commandId": "string", "decisionId": "string", "actionKey": "string",
		}},
		{"runtime.playback.action_interrupted", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"commandId": "string", "decisionId": "string",
		}},
		{"runtime.playback.action_failed", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"commandId": "string", "decisionId": "string", "errorClass": "string",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}

func registerRuntimeSchemas() {
	schemas := []struct {
		eventType   string
		reliability EventReliability
		ttl         time.Duration
		fields      map[string]string
	}{
		{"runtime.connected", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"runtimeId": "string", "petInstanceId": "string",
		}},
		{"runtime.disconnected", ReliabilityRecoverable, 60 * time.Second, map[string]string{
			"runtimeId": "string", "petInstanceId": "string",
		}},
		{"installation.active.changed", ReliabilityDurable, 0, map[string]string{
			"installationId": "string", "petInstanceId": "string", "stateRevision": "int64",
		}},
		{"manual.action.requested", ReliabilityDurable, 0, map[string]string{
			"actionKey": "string", "force": "bool",
		}},
	}
	for _, s := range schemas {
		RegisterSchema(&EventSchemaDef{
			EventType:     s.eventType,
			Reliability:   s.reliability,
			DefaultTTL:    s.ttl,
			AllowedFields: s.fields,
			SchemaVersion: 1,
		})
	}
}
