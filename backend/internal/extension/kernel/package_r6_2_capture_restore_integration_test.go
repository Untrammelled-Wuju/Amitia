package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func TestR62RealCaptureRestoreClosesAggregateHash(t *testing.T) {
	const extensionID = "r62real"

	sourceDB, table := newR61CaptureDatabase(t, extensionID, []r61Row{
		{EntityID: "record-1", Value: "first", Amount: 1},
		{EntityID: "record-2", Value: "second", Amount: 2},
	})

	assertR62RealCaptureRestoreClosesAggregateHash(t, extensionID, sourceDB, table, "entity_id")
}

func TestR62RealCaptureRestoreWithIDColumnClosesAggregateHash(t *testing.T) {
	const extensionID = "r62realid"

	sourceDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sourceDB.Close()
	})

	table := migration.ExtensionNamespacePrefix(extensionID) + "records"
	_, err = sourceDB.Exec(fmt.Sprintf(`CREATE TABLE %s (
		id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL,
		amount INTEGER NOT NULL
	)`, quoteIdentifier(table)))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range []r61Row{
		{EntityID: "record-1", Value: "first", Amount: 1},
		{EntityID: "record-2", Value: "second", Amount: 2},
	} {
		_, err = sourceDB.Exec(
			fmt.Sprintf("INSERT INTO %s (id, entity_value, amount) VALUES (?, ?, ?)", quoteIdentifier(table)),
			row.EntityID,
			row.Value,
			row.Amount,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	assertR62RealCaptureRestoreClosesAggregateHash(t, extensionID, sourceDB, table, "id")
}

func assertR62RealCaptureRestoreClosesAggregateHash(
	t *testing.T,
	extensionID string,
	sourceDB *sql.DB,
	table string,
	idColumn string,
) {
	t.Helper()

	captured, err := captureUserDataTableSnapshot(context.Background(), sourceDB, extensionID, table)
	if err != nil {
		t.Fatalf("capture real table: %v", err)
	}

	restoreDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "restore.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restoreDB.Close()

	_, err = restoreDB.Exec(fmt.Sprintf(`CREATE TABLE %s (
		%s TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL,
		amount INTEGER NOT NULL
	)`, quoteIdentifier(table), quoteIdentifier(idColumn)))
	if err != nil {
		t.Fatal(err)
	}

	store := NewUserDataSnapshotStore(restoreDB)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure restore schema: %v", err)
	}

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{table},
		RecordCounts:   map[string]int64{table: captured.count},
		DataExports:    map[string]string{table: captured.jsonl},
		TableManifests: map[string]UserDataTableSnapshotManifest{table: captured.manifest},
	}
	rawState, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}

	const operationID = "operation-r62-real-capture-restore"
	if err := store.RestoreUserDataFromSnapshot(context.Background(), extensionID, operationID, string(rawState)); err != nil {
		t.Fatalf("restore real capture: %v", err)
	}
	if err := store.VerifyUserDataRestore(context.Background(), operationID); err != nil {
		t.Fatalf("verify real capture restore: %v", err)
	}

	databaseHash, err := store.computeAggregateHashFromDB(context.Background(), table)
	if err != nil {
		t.Fatal(err)
	}
	if databaseHash != captured.manifest.AggregateHash {
		t.Fatalf("real capture/restore aggregate mismatch: manifest=%s database=%s", captured.manifest.AggregateHash, databaseHash)
	}
}
