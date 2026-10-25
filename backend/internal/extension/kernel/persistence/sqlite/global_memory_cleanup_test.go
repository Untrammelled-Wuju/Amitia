package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func TestGlobalMemoryCleanupRemovesResourcesAndProjectionState(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
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
	for index := MigrationCount() - 3; index < MigrationCount(); index++ {
		if _, err := db.Exec(schemaMigrations[index]); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_owned_resources WHERE role_id='@global-memory'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("global resources=%d err=%v", count, err)
	}
}
