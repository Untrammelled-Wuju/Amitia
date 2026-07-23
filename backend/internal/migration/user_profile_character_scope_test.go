package migration

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserProfileCharacterScopeMigrationUpgradesLegacyTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-profile-character-scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE user_profiles (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL DEFAULT 'default',
		category TEXT NOT NULL,
		attribute_name TEXT NOT NULL,
		attribute_value TEXT NOT NULL,
		confidence INTEGER DEFAULT 50,
		source_conv_id TEXT DEFAULT '',
		verified_at TEXT DEFAULT '',
		created_at TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_user_profiles_uid_cat_attr ON user_profiles(user_id, category, attribute_name)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_profiles (id, user_id, category, attribute_name, attribute_value) VALUES (?, ?, ?, ?, ?)", "profile-1", "1", "memory", "initial_memory", "自然随意").Error; err != nil {
		t.Fatal(err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	migration := UserProfileCharacterScopeMigration()
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}

	for _, column := range []string{"character_id", "source"} {
		if !db.Migrator().HasColumn("user_profiles", column) {
			t.Fatalf("missing column %s", column)
		}
	}

	var indexColumns []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA index_info(idx_user_profiles_uid_cat_attr)").Scan(&indexColumns).Error; err != nil {
		t.Fatal(err)
	}
	expected := []string{"user_id", "character_id", "category", "attribute_name"}
	if len(indexColumns) != len(expected) {
		t.Fatalf("index column count = %d, want %d", len(indexColumns), len(expected))
	}
	for i, column := range expected {
		if indexColumns[i].Name != column {
			t.Fatalf("index column %d = %s, want %s", i, indexColumns[i].Name, column)
		}
	}

	var row struct {
		AttributeValue string `gorm:"column:attribute_value"`
		CharacterID    string `gorm:"column:character_id"`
		Source         string `gorm:"column:source"`
	}
	if err := db.Table("user_profiles").Where("id = ?", "profile-1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.AttributeValue != "自然随意" || row.CharacterID != "" || row.Source != "" {
		t.Fatalf("unexpected migrated row: %+v", row)
	}
}
