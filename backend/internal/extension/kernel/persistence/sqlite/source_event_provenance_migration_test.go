package sqlite

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

func TestSourceEventHostProvenanceUpgradePreservesCanonicalEventAndChecksums(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	last := MigrationCount()
	var checksum string
	if err := db.QueryRowContext(t.Context(), "SELECT checksum FROM schema_migrations WHERE version=?", last-1).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	record := event.OutboxRecord{OutboxID: "legacy", EventID: "legacy", EventTypeID: "extension.source.updated", ProducerID: "source", ProducerType: event.EventProducerTypeExtension, Payload: []byte(`{"private":"old"}`), PayloadHash: "old", IdempotencyKey: "old", OccurredAt: time.Now()}
	if err := event.NewOutboxRepository(db).Enqueue(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "DROP TABLE extension_event_host_provenance"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "DELETE FROM schema_migrations WHERE version=?", last); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	old, err := event.NewOutboxRepository(db).Get(t.Context(), "legacy")
	if err != nil || string(old.Payload) != string(record.Payload) || len(old.HostProvenance) != 0 {
		t.Fatalf("legacy data changed: %+v %v", old, err)
	}
	var actual string
	if err := db.QueryRowContext(t.Context(), "SELECT checksum FROM schema_migrations WHERE version=?", last-1).Scan(&actual); err != nil || actual != checksum {
		t.Fatal("published migration checksum changed")
	}
}
