package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

func TestSourceEventContractUsesCanonicalOutboxWithoutGlobalRegistration(t *testing.T) {
	db := setupSameIDTestDB(t)
	service, err := event.NewService(event.DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatal(err)
	}
	bridge := event.NewRuntimeBridge(service)
	definition := event.EventTypeDefinition{EventTypeID: "extension.source.only", Version: 1, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10, RiskLevel: event.RiskLevelLow, OrderingPolicy: event.OrderingNone, PayloadSchema: json.RawMessage(`{"type":"object","required":["private"],"properties":{"private":{"type":"string"}}}`), ProducerPolicy: event.EventProducerPolicy{AllowedProducers: []string{"extension"}, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 32 << 10}}
	definition.DefinitionHash = definition.Hash()
	for _, invalid := range []event.EventTypeDefinition{
		func() event.EventTypeDefinition { d := definition; d.DefinitionHash = "forged"; return d }(),
		func() event.EventTypeDefinition {
			d := definition
			d.EventTypeID = "extension.foreign.only"
			d.DefinitionHash = d.Hash()
			return d
		}(),
	} {
		if _, err := bridge.PublishSourceContract(t.Context(), invalid, "source", json.RawMessage(`{"private":"<>&中文"}`), event.PublishOptions{}); err == nil {
			t.Fatal("invalid Source schema accepted")
		}
	}
	if _, err := bridge.PublishSourceContract(t.Context(), definition, "source", json.RawMessage(`{"private":4}`), event.PublishOptions{}); err == nil {
		t.Fatal("Source schema not applied to actual input")
	}
	for _, schema := range []json.RawMessage{json.RawMessage(`{"$ref":"file:///private/secret.json"}`), json.RawMessage(`{"$ref":"https://remote.invalid/schema.json"}`), json.RawMessage(`{"type":4}`)} {
		invalid := definition
		invalid.PayloadSchema = schema
		invalid.DefinitionHash = invalid.Hash()
		if _, err := bridge.PublishSourceContract(t.Context(), invalid, "source", json.RawMessage(`{"private":"text"}`), event.PublishOptions{}); err == nil {
			t.Fatal("unsafe or invalid Source schema accepted")
		}
	}
	result, err := bridge.PublishSourceContract(t.Context(), definition, "source", json.RawMessage(`{"private":"<>&中文"}`), event.PublishOptions{ProducerGeneration: 7, ProducerModuleID: "module", AggregateID: "run", AggregateType: "source-task", Metadata: json.RawMessage(`{"amitiaSourceTaskContract":{"source":"device","schemaHash":"fixed"}}`)})
	if err != nil || !result.Accepted || result.OutboxID == "" {
		t.Fatalf("durable contract publish: %+v %v", result, err)
	}
	if _, err := service.GetEventType(t.Context(), definition.EventTypeID, 1); err == nil {
		t.Fatal("Source schema polluted Core global registry")
	}
	var count int
	var hash string
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*),definition_hash FROM extension_event_outbox WHERE aggregate_id=?", "run").Scan(&count, &hash); err != nil || count != 1 || hash != definition.DefinitionHash {
		t.Fatalf("canonical outbox contract identity: %d %s %v", count, hash, err)
	}
	record, err := service.GetOutboxRecord(t.Context(), result.OutboxID)
	if err != nil {
		t.Fatal(err)
	}
	planner := event.NewDeliveryPlanner(nil, nil, nil, nil, event.NewOutboxRepository(db), nil)
	if err := planner.PlanDeliveries(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	record, err = service.GetOutboxRecord(t.Context(), result.OutboxID)
	if err != nil || record.Status != event.OutboxStatusDispatched {
		t.Fatalf("Source contract invoked Core subscription route: %+v %v", record, err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := bridge.PublishSourceContract(cancelled, definition, "source", json.RawMessage(`{"private":"late"}`), event.PublishOptions{}); err == nil {
		t.Fatal("cancelled Source contract persisted")
	}
	if err := bridge.RegisterExtensionEvents(t.Context(), "source", 7, []event.EventTypeDefinition{definition}, nil); err != nil {
		t.Fatal(err)
	}
	contracts, err := bridge.InstalledEventContracts(t.Context(), "source", 7)
	if err != nil || len(contracts) != 1 {
		t.Fatalf("fixed installed contracts: %v %v", contracts, err)
	}
	if _, err := bridge.InstalledEventContracts(t.Context(), "source", 6); err == nil {
		t.Fatal("stale Source generation accepted")
	}
	if err := bridge.UnregisterExtensionEvents(t.Context(), "source"); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.InstalledEventContracts(t.Context(), "source", 7); err == nil {
		t.Fatal("uninstalled Source contract accepted")
	}
}
