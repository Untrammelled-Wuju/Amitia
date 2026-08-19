package migration

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSessionTimestampCompatibilityMigrationRebuildsTextColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:session-timestamp-compatibility?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE auth_sessions (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, username TEXT NOT NULL, role TEXT NOT NULL, token_hash TEXT NOT NULL, device_name TEXT, ip_address TEXT, user_agent TEXT, last_active_at TEXT, expires_at TEXT, created_at TEXT, public_id TEXT UNIQUE, status TEXT, revision INTEGER, absolute_expires_at TEXT, revoked_at TEXT, revoke_reason TEXT, last_refreshed_at TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE desktop_pet_local_sessions (id TEXT PRIMARY KEY, user_id TEXT, desktop_instance_id TEXT, token_hash TEXT, status TEXT, created_at TEXT, expires_at TEXT, last_used_at TEXT, revoked_at TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO auth_sessions (id, user_id, username, role, token_hash, created_at, expires_at, status, revision) VALUES (1, 1, 'u', 'admin', 'hash', '2026-08-20 00:00:00+00:00', '2026-08-21 00:00:00+00:00', 'active', 1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO desktop_pet_local_sessions (id, user_id, desktop_instance_id, token_hash, status, created_at, expires_at, last_used_at) VALUES ('s', '1', 'd', 'hash', 'active', '2026-08-20 00:00:00+00:00', '2026-08-21 00:00:00+00:00', '2026-08-20 00:00:00+00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{SessionTimestampCompatibilityMigration()}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"auth_sessions", "desktop_pet_local_sessions"} {
		var columnType string
		if err := db.Raw(fmt.Sprintf("SELECT type FROM pragma_table_info('%s') WHERE name = 'expires_at'", table)).Scan(&columnType).Error; err != nil {
			t.Fatal(err)
		}
		if columnType != "DATETIME" {
			t.Fatalf("%s.expires_at type = %q, want DATETIME", table, columnType)
		}
	}
}
