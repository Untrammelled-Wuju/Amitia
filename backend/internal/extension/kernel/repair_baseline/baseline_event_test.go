package repair_baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type baselineSchemaRegistry struct {
	types map[event.EventTypeID]event.EventTypeDefinition
}

func (r *baselineSchemaRegistry) RegisterEventType(_ context.Context, def event.EventTypeDefinition) error {
	if r.types == nil {
		r.types = make(map[event.EventTypeID]event.EventTypeDefinition)
	}
	r.types[def.EventTypeID] = def
	return nil
}

func (r *baselineSchemaRegistry) GetEventType(_ context.Context, typeID event.EventTypeID, _ int) (event.EventTypeDefinition, error) {
	if r.types == nil {
		r.types = make(map[event.EventTypeID]event.EventTypeDefinition)
	}
	if def, ok := r.types[typeID]; ok {
		return def, nil
	}
	return event.EventTypeDefinition{}, fmt.Errorf("event: type %s not registered", typeID)
}

func (r *baselineSchemaRegistry) ListEventTypes(_ context.Context) ([]event.EventTypeDefinition, error) {
	out := make([]event.EventTypeDefinition, 0, len(r.types))
	for _, def := range r.types {
		out = append(out, def)
	}
	return out, nil
}

func (r *baselineSchemaRegistry) ListByNamespace(_ context.Context, _ string) ([]event.EventTypeDefinition, error) {
	return nil, nil
}

func (r *baselineSchemaRegistry) IsRegistered(_ context.Context, _ event.EventTypeID, _ int) bool {
	return false
}

func newBaselineRegistry(t *testing.T) (*event.EventSubscriptionRegistry, event.EventTypeID) {
	t.Helper()
	reg := &baselineSchemaRegistry{}
	typeID := event.EventTypeID("baseline.event.repair")
	schema, _ := json.Marshal(map[string]any{"type": "object", "properties": map[string]any{"msg": map[string]any{"type": "string"}}})
	if err := reg.RegisterEventType(context.Background(), event.EventTypeDefinition{
		EventTypeID:      typeID,
		Version:          1,
		Description:      "Baseline Repair Event",
		PayloadSchema:    schema,
		MaxPayloadBytes:  4096,
		MaxMetadataBytes: 1024,
		SubscriberPolicy: event.EventSubscriberPolicy{
			AllowThirdParty:     true,
			AllowedFilterFields: []string{"msg"},
		},
	}); err != nil {
		t.Fatalf("register event type: %v", err)
	}
	return event.NewEventSubscriptionRegistry(reg, 16), typeID
}

func TestBaseline_Event_EffectiveStateNotHardcoded(t *testing.T) {
	reg, typeID := newBaselineRegistry(t)

	def := event.EventSubscriptionDefinition{
		ContributionID:    "baseline.sub.repair",
		ExtensionID:       "com.amitia.baseline/repair",
		ModuleID:          "main",
		EventTypeID:       typeID,
		EventVersionRange: "1",
		Entry:             "onBaselineEvent",
		Generation:        1,
		Enabled:           true,
	}
	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("register subscription: %v", err)
	}

	sub, ok := reg.Get(context.Background(), def.ContributionID)
	if !ok {
		t.Fatalf("subscription not found after Register")
	}

	if sub.Effective.PermissionGranted {
		t.Fatalf("PermissionGranted must NOT be hardcoded true without a real grant; the registry must resolve effective state from Permission Broker")
	}
	if sub.Effective.ScopeValid {
		t.Fatalf("ScopeValid must NOT be hardcoded true without a real scope snapshot")
	}
	if sub.Effective.DependenciesReady {
		t.Fatalf("DependenciesReady must NOT be hardcoded true without a real dependency check")
	}
	if sub.Effective.RuntimeAvailable {
		t.Fatalf("RuntimeAvailable must NOT be hardcoded true without a real runtime readiness check")
	}
}

func TestBaseline_Event_OldGenerationDeliveryMustCancel(t *testing.T) {
	reg, typeID := newBaselineRegistry(t)

	oldDef := event.EventSubscriptionDefinition{
		ContributionID:    "baseline.sub.gen1",
		ExtensionID:       "com.amitia.baseline/gen",
		ModuleID:          "main",
		EventTypeID:       typeID,
		EventVersionRange: "1",
		Entry:             "onBaselineEvent",
		Generation:        1,
		Enabled:           true,
	}
	if err := reg.Register(context.Background(), oldDef); err != nil {
		t.Fatalf("register gen1 subscription: %v", err)
	}

	newDef := event.EventSubscriptionDefinition{
		ContributionID:    "baseline.sub.gen2",
		ExtensionID:       "com.amitia.baseline/gen",
		ModuleID:          "main",
		EventTypeID:       typeID,
		EventVersionRange: "1",
		Entry:             "onBaselineEvent",
		Generation:        2,
		Enabled:           true,
	}
	if err := reg.UpdateGeneration(context.Background(), oldDef.ExtensionID, 2, []event.EventSubscriptionDefinition{newDef}); err != nil {
		t.Fatalf("update generation: %v", err)
	}

	if _, stillExists := reg.Get(context.Background(), oldDef.ContributionID); stillExists {
		t.Fatalf("old generation subscription must be removed from registry after UpdateGeneration")
	}

	if cnt := reg.CountByType(typeID); cnt != 1 {
		t.Fatalf("byType index must contain exactly 1 subscription after generation update, got %d (index removal is broken)", cnt)
	}

	payload, _ := json.Marshal(map[string]any{"msg": "hi"})
	resolved, err := reg.Resolve(context.Background(), event.EventEnvelope{
		EventID:      "evt-baseline-1",
		EventTypeID:  typeID,
		EventVersion: 1,
		AggregateID:  "agg-1",
		OccurredAt:   time.Now().UTC(),
		Payload:      payload,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, r := range resolved {
		if r.Definition.Generation != 2 {
			t.Fatalf("resolved subscription generation=%d, expected 2 (old generation leaked)", r.Definition.Generation)
		}
	}
}

var _ = errors.New
