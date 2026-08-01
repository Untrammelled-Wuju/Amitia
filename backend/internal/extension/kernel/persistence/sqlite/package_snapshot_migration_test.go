package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func TestPackageRollbackSnapshotColumnsMigrateFromLegacyTable(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "snapshot-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE extension_package_rollback_points (
		rollback_point_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		source_version TEXT NOT NULL,
		source_generation INTEGER NOT NULL,
		artifact_id TEXT NOT NULL,
		definition_snapshot_json TEXT NOT NULL,
		module_snapshot_json TEXT NOT NULL,
		contribution_snapshot_json TEXT NOT NULL,
		permission_snapshot_json TEXT NOT NULL,
		scope_snapshot_json TEXT NOT NULL,
		config_snapshot_id TEXT NOT NULL DEFAULT '',
		installed_path TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"config_snapshot_json": false, "secret_refs_json": false, "resource_snapshot_json": false, "migration_state_snapshot_json": false, "user_data_migration_state_json": false, "snapshot_hash": false, "retention_state": false, "retention_until": false, "source_operation_id": false}
	rows, err := db.Query(`PRAGMA table_info(extension_package_rollback_points)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if _, exists := wanted[name]; exists {
			wanted[name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Fatalf("migration did not add %s", name)
		}
	}
}

func TestResourceQuarantineColumnsMigrateFromLegacyTable(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "quarantine-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE extension_package_resource_quarantine (
		quarantine_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		logical_path TEXT NOT NULL DEFAULT '',
		content_hash TEXT NOT NULL DEFAULT '',
		storage_reference TEXT NOT NULL DEFAULT '',
		quarantine_path TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL DEFAULT 'quarantined',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (quarantine_id)
	)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"size": false, "namespace_hash": false, "original_path": false, "content_storage_reference": false}
	rows, err := db.Query(`PRAGMA table_info(extension_package_resource_quarantine)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if _, exists := wanted[name]; exists {
			wanted[name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Fatalf("migration did not add %s to resource_quarantine", name)
		}
	}
}

func TestUserDataRestoreJournalColumnsMigrateFromLegacyTable(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "journal-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE extension_package_user_data_restore_journal (
		journal_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		table_name TEXT NOT NULL,
		total_rows INTEGER NOT NULL DEFAULT 0,
		imported_rows INTEGER NOT NULL DEFAULT 0,
		state TEXT NOT NULL DEFAULT 'pending',
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		error_detail TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (operation_id, table_name)
	)`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"applied_count": false, "cursor": false, "batch_hash": false, "namespace_hash": false}
	rows, err := db.Query(`PRAGMA table_info(extension_package_user_data_restore_journal)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if _, exists := wanted[name]; exists {
			wanted[name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Fatalf("migration did not add %s to user_data_restore_journal", name)
		}
	}
}
