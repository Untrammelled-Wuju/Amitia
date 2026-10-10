package event

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
)

func TestNativeHostProvenanceIsAtomicAndOutsideZeroPluginMetadataBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(t.Context(), "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatal(err)
	}
	bridge := NewRuntimeBridge(service)
	definition := EventTypeDefinition{EventTypeID: "extension.source.updated", Version: 1, MaxPayloadBytes: 64 << 10, MaxMetadataBytes: 0, ProducerPolicy: EventProducerPolicy{AllowedProducers: []string{"extension"}}, RiskLevel: RiskLevelLow}
	definition.DefinitionHash = definition.Hash()
	payload := json.RawMessage(`{"private":"<>&中文"}`)
	hash := sha256.Sum256(payload)
	opts := PublishOptions{ProducerGeneration: 7, AggregateID: "run", AggregateType: "source-task", ScopeSnapshotID: "scope", TraceID: "native"}
	provenance := SourceEventProvenance{ScopeSnapshotID: "scope", SourceDeviceID: "source-device", TaskRunID: "run", TaskGeneration: 1, AttemptID: "attempt", RequestID: "native", InstalledGeneration: 7, SchemaHash: definition.Hash(), PayloadHash: hex.EncodeToString(hash[:])}
	result, err := bridge.PublishSourceContract(t.Context(), definition, "source", payload, opts, provenance)
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.GetOutboxRecord(t.Context(), result.OutboxID)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.HostProvenance) == 0 || len(record.Metadata) != 0 || bytes.Contains(record.HostProvenance, []byte("private")) {
		t.Fatal("host provenance occupied plugin metadata or copied payload")
	}
	claimed, err := NewOutboxRepository(db).ClaimNext(t.Context(), "reader", time.Second, 10)
	if err != nil || len(claimed) != 1 || !bytes.Equal(claimed[0].HostProvenance, record.HostProvenance) {
		t.Fatalf("single-connection provenance claim: %v %v", claimed, err)
	}
	planner := NewDeliveryPlanner(nil, nil, nil, nil, NewOutboxRepository(db), nil)
	if err := planner.PlanDeliveries(t.Context(), claimed[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `CREATE TRIGGER fail_native_provenance BEFORE INSERT ON extension_event_host_provenance BEGIN SELECT RAISE(ABORT,'test stage failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.PublishSourceContract(t.Context(), definition, "source", payload, opts, provenance); err == nil {
		t.Fatal("failed provenance stage did not rollback")
	}
	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_outbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("partial event survived failed provenance: %d %v", count, err)
	}
	if _, err := db.ExecContext(t.Context(), "DROP TRIGGER fail_native_provenance"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	service, err = NewService(DefaultServiceConfig().WithDB(db))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := service.GetOutboxRecord(t.Context(), result.OutboxID)
	if err != nil || !bytes.Equal(reopened.HostProvenance, record.HostProvenance) {
		t.Fatalf("reopened host provenance lost: %+v %v", reopened, err)
	}
	if _, err := NewOutboxRepository(db).DeleteOlderThan(t.Context(), time.Now().Add(time.Hour), OutboxStatusDispatched); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM extension_event_host_provenance").Scan(&count); err != nil || count != 0 {
		t.Fatalf("host provenance orphan with FK disabled: %d %v", count, err)
	}
	if err := service.RegisterEventType(t.Context(), definition); err != nil {
		t.Fatal(err)
	}
	bridge = NewRuntimeBridge(service)
	spoof := PublishOptions{hostProvenance: json.RawMessage(`{"sourceDeviceId":"forged"}`)}
	ordinary, err := bridge.PublishFromRuntime(context.Background(), "source", definition.EventTypeID, 1, payload, spoof)
	if err != nil {
		t.Fatal(err)
	}
	ordinaryRecord, err := service.GetOutboxRecord(t.Context(), ordinary.OutboxID)
	if err != nil || len(ordinaryRecord.HostProvenance) != 0 {
		t.Fatal("ordinary plugin injected trusted provenance")
	}
	if _, err := bridge.PublishFromRuntime(t.Context(), "source", definition.EventTypeID, 1, payload, PublishOptions{Metadata: json.RawMessage(`{"amitiaSourceTaskContract":{}}`)}); err == nil {
		t.Fatal("ordinary plugin forged legacy host marker")
	}
	if _, err := bridge.PublishSourceContract(t.Context(), definition, "source", payload, PublishOptions{Metadata: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("zero plugin metadata budget was overridden")
	}
}
