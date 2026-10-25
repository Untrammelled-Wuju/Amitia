package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func TestGlobalMemoryCleanupRemovesResourcesAndProjectionState(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"value": "legacy"})
	if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,source_id,revision,deleted,body,updated_at) VALUES('owner','memory','memory','@global-memory','',1,0,?,'now')`, body); err != nil {
		t.Fatal(err)
	}
	for _, statement := range schemaMigrations {
		if !strings.HasPrefix(statement, "DELETE FROM kernel_device_owned_") {
			continue
		}
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE role_id='@global-memory'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("global resources=%d err=%v", count, err)
	}
}

func TestGlobalMemoryCleanupNormalizesLegacyChecksum(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kernel-checksum.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	version := 0
	for index, statement := range schemaMigrations {
		if strings.HasPrefix(statement, "DELETE FROM kernel_device_owned_") {
			version = index + 1
			break
		}
	}
	if version == 0 {
		t.Fatal("missing global memory cleanup migration")
	}
	if _, err := db.Exec(
		`UPDATE schema_migrations SET checksum = 'legacy-checksum' WHERE version = ?`,
		version,
	); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var checksum string
	if err := db.QueryRow(
		`SELECT checksum FROM schema_migrations WHERE version = ?`,
		version,
	).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	if checksum != computeChecksum(schemaMigrations[version-1]) {
		t.Fatalf("checksum = %s", checksum)
	}
}
