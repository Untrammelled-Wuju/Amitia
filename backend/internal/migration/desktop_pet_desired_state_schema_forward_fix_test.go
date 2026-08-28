package migration

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDesktopPetDesiredStateSchemaForwardFixMigratesLegacyRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:desired-state-forward-fix?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	legacy := []string{
		`CREATE TABLE desktop_pet_runtime_desired_states (
			id TEXT PRIMARY KEY,
			installation_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			device_id TEXT NOT NULL DEFAULT '',
			desired_enabled INTEGER NOT NULL DEFAULT 0,
			desired_visible INTEGER NOT NULL DEFAULT 0,
			desired_release_id TEXT NOT NULL DEFAULT '',
			desired_action_key TEXT NOT NULL DEFAULT '',
			position_x REAL,
			position_y REAL,
			scale REAL NOT NULL DEFAULT 1.0,
			opacity REAL NOT NULL DEFAULT 1.0,
			always_on_top INTEGER NOT NULL DEFAULT 1,
			click_through_mode TEXT NOT NULL DEFAULT 'off',
			position_policy TEXT NOT NULL DEFAULT '',
			revision INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT DEFAULT '',
			created_at TEXT DEFAULT '',
			UNIQUE(installation_id)
		)`,
		`CREATE TABLE desktop_pet_installations (
			id TEXT PRIMARY KEY,
			pet_id TEXT NOT NULL DEFAULT '',
			current_release_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE desktop_pet_device_active_installation_bindings (
			user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			installation_id TEXT NOT NULL DEFAULT '',
			PRIMARY KEY(user_id, device_id)
		)`,
		`CREATE TABLE desktop_pet_device_desired_revision_counters (
			user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			current_revision INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT DEFAULT '',
			PRIMARY KEY(user_id, device_id)
		)`,
		`CREATE TABLE desktop_pet_runtime_desired_state_outbox (
			event_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			device_id TEXT NOT NULL,
			installation_id TEXT NOT NULL DEFAULT '',
			desired_revision INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, stmt := range legacy {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO desktop_pet_installations(id, pet_id, current_release_id) VALUES
		('inst-old', 'pet-old', 'release-old'), ('inst-active', 'pet-active', 'release-active')`).Error; err != nil {
		t.Fatalf("insert installations: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(id, installation_id, user_id, device_id, desired_enabled, desired_visible, desired_release_id, desired_action_key, revision, updated_at)
		VALUES
		('desired-old', 'inst-old', 'u', 'd', 1, 1, 'release-old', 'idle', 8, '2026-08-01 00:00:00'),
		('desired-active', 'inst-active', 'u', 'd', 1, 1, 'release-active', 'wave', 7, '2026-08-02 00:00:00')`).Error; err != nil {
		t.Fatalf("insert desired rows: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_device_active_installation_bindings(user_id, device_id, installation_id)
		VALUES ('u', 'd', 'inst-active')`).Error; err != nil {
		t.Fatalf("insert active binding: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_device_desired_revision_counters(user_id, device_id, current_revision, updated_at)
		VALUES ('u', 'd', 3, '2026-07-01 00:00:00')`).Error; err != nil {
		t.Fatalf("insert legacy revision counter: %v", err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_state_outbox
		(event_id, user_id, device_id, installation_id, desired_revision, status, last_error) VALUES
		('outbox-stale', 'u', 'd', 'inst-old', 8, 'pending', ''),
		('outbox-current', 'u', 'd', 'inst-active', 7, 'pending', '')`).Error; err != nil {
		t.Fatalf("insert legacy outbox: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{DesktopPetDesiredStateSchemaForwardFixMigration()}); err != nil {
		t.Fatalf("apply desired-state forward fix: %v", err)
	}

	for _, column := range []string{
		"runtime_id", "pet_id", "release_id", "settings_snapshot_json", "settings_revision",
		"desired_revision", "desired_hash", "reason", "operation_id",
	} {
		if !db.Migrator().HasColumn("desktop_pet_runtime_desired_states", column) {
			t.Fatalf("canonical desired-state column missing: %s", column)
		}
	}

	var rows []struct {
		ID              string `gorm:"column:id"`
		InstallationID  string `gorm:"column:installation_id"`
		PetID           string `gorm:"column:pet_id"`
		ReleaseID       string `gorm:"column:release_id"`
		DesiredRevision int64  `gorm:"column:desired_revision"`
	}
	if err := db.Table("desktop_pet_runtime_desired_states").Where("user_id = ? AND device_id = ?", "u", "d").Find(&rows).Error; err != nil {
		t.Fatalf("read migrated desired state: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one authoritative desired row, got %d", len(rows))
	}
	row := rows[0]
	if row.InstallationID != "inst-active" || row.PetID != "pet-active" || row.ReleaseID != "release-active" || row.DesiredRevision != 7 {
		t.Fatalf("wrong authoritative row after migration: %#v", row)
	}

	var counter struct {
		CurrentRevision int64 `gorm:"column:current_revision"`
	}
	if err := db.Table("desktop_pet_device_desired_revision_counters").Where("user_id = ? AND device_id = ?", "u", "d").Take(&counter).Error; err != nil {
		t.Fatalf("read migrated revision counter: %v", err)
	}
	if counter.CurrentRevision != 8 {
		t.Fatalf("revision counter did not preserve historical max: got=%d want=8", counter.CurrentRevision)
	}

	var staleStatus, staleError, currentStatus string
	if err := db.Table("desktop_pet_runtime_desired_state_outbox").Select("status, last_error").Where("event_id = ?", "outbox-stale").Row().Scan(&staleStatus, &staleError); err != nil {
		t.Fatalf("read stale outbox: %v", err)
	}
	if err := db.Table("desktop_pet_runtime_desired_state_outbox").Select("status").Where("event_id = ?", "outbox-current").Row().Scan(&currentStatus); err != nil {
		t.Fatalf("read current outbox: %v", err)
	}
	if staleStatus != "failed" || staleError != "superseded_by_desired_state_schema_forward_fix" || currentStatus != "pending" {
		t.Fatalf("outbox authority migration incorrect: stale=(%s,%s) current=%s", staleStatus, staleError, currentStatus)
	}

	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states
		(id, installation_id, user_id, device_id) VALUES ('duplicate', 'other', 'u', 'd')`).Error; err == nil {
		t.Fatal("device-scoped desired-state uniqueness was not enforced")
	}
}
