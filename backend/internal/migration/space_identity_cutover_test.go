package migration

import (
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/spaceidentity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSpaceIdentityCutoverRemovesAccountIdentityAndMigratesTemporalScopes(t *testing.T) {
	if _, err := spaceidentity.InitializeDefault(filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	canonical := spaceidentity.DefaultSpaceID()
	if canonical == "" {
		t.Fatal("canonical space id is empty")
	}

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`PRAGMA foreign_keys = ON`).Error; err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE auth_users (id TEXT PRIMARY KEY, username TEXT, nickname TEXT, user_label TEXT, bio TEXT, is_active INTEGER)`,
		`CREATE TABLE auth_sessions (id TEXT PRIMARY KEY)`,
		`CREATE TABLE auth_refresh_tokens (id TEXT PRIMARY KEY)`,
		`CREATE TABLE auth_login_guards (id TEXT PRIMARY KEY)`,
		`CREATE TABLE auth_recovery_codes (id TEXT PRIMARY KEY)`,
		`CREATE TABLE auth_recovery_grants (id TEXT PRIMARY KEY)`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, user_id TEXT NOT NULL DEFAULT '' REFERENCES auth_users(id))`,
		`CREATE TABLE temporal_profiles (id TEXT PRIMARY KEY, owner_type TEXT NOT NULL, owner_id TEXT NOT NULL)`,
		`CREATE TABLE temporal_anchors (id TEXT PRIMARY KEY, scope_type TEXT NOT NULL DEFAULT 'user', user_id TEXT NOT NULL DEFAULT '', FOREIGN KEY(user_id) REFERENCES auth_users(id) ON DELETE RESTRICT)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO auth_users(id,username,nickname,user_label,bio,is_active) VALUES ('legacy','legacy','Legacy Name','legacy-label','legacy-bio',1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO conversations(id,user_id) VALUES ('conv','legacy')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO temporal_profiles(id,owner_type,owner_id) VALUES ('p','user','legacy')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO temporal_anchors(id,scope_type,user_id) VALUES ('a','user','legacy')`).Error; err != nil {
		t.Fatal(err)
	}

	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{SpaceIdentityCutoverMigration()}); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"auth_users", "auth_sessions", "auth_refresh_tokens", "auth_login_guards", "auth_recovery_codes", "auth_recovery_grants"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("legacy account table still exists: %s", table)
		}
	}
	if db.Migrator().HasColumn("conversations", "user_id") || !db.Migrator().HasColumn("conversations", "space_id") {
		t.Fatal("conversation ownership column was not cut over to space_id")
	}

	var conversationSpace string
	if err := db.Raw(`SELECT space_id FROM conversations WHERE id='conv'`).Scan(&conversationSpace).Error; err != nil {
		t.Fatal(err)
	}
	if conversationSpace != canonical {
		t.Fatalf("conversation space = %q, want canonical %q", conversationSpace, canonical)
	}
	var ownerType, ownerID string
	if err := db.Raw(`SELECT owner_type, owner_id FROM temporal_profiles WHERE id='p'`).Row().Scan(&ownerType, &ownerID); err != nil {
		t.Fatal(err)
	}
	if ownerType != "space" {
		t.Fatalf("temporal profile owner_type = %q, want space", ownerType)
	}
	var scopeType, anchorSpace string
	if err := db.Raw(`SELECT scope_type, space_id FROM temporal_anchors WHERE id='a'`).Row().Scan(&scopeType, &anchorSpace); err != nil {
		t.Fatal(err)
	}
	if scopeType != "space" || anchorSpace != canonical {
		t.Fatalf("temporal anchor = (%q,%q), want (space,%q)", scopeType, anchorSpace, canonical)
	}
	for _, table := range []string{"conversations", "temporal_anchors"} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM pragma_foreign_key_list(?) WHERE lower("table") IN ('auth_users','auth_sessions','auth_refresh_tokens','auth_login_guards','auth_recovery_codes','auth_recovery_grants')`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("legacy account foreign key still exists on %s", table)
		}
	}
}
