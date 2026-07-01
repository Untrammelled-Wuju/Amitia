package character

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func TestGetRuntimeProfileLoadsCompleteConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&Character{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(app.NewAppContext(db, nil))
	if err := repo.Create(&Character{
		ID:                "char-1",
		Name:              "Amitia",
		Identity:          "心理模拟伙伴",
		Personality:       "温和、敏锐",
		SpeakingStyle:     "简洁回应",
		RelationshipStyle: "亲密但有边界",
		SystemPrompt:      "保持角色一致",
		BoundaryRules:     "不越界承诺",
		PersonalityConfig: `{"openness":72}`,
		ChatStyleConfig:   `{"pace":"slow"}`,
		SceneRules:        `{"place":"library"}`,
		IsDefault:         1,
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := repo.GetRuntimeProfile("")
	if err != nil {
		t.Fatal(err)
	}
	if profile.CharacterID != "char-1" || profile.Name != "Amitia" {
		t.Fatalf("unexpected runtime profile: %#v", profile)
	}
	if profile.PersonalityConfig["openness"].(float64) != 72 {
		t.Fatalf("personality config not loaded: %#v", profile.PersonalityConfig)
	}
	if profile.ChatStyleConfig["pace"] != "slow" || profile.SceneRules["place"] != "library" {
		t.Fatalf("runtime json not loaded: %#v %#v", profile.ChatStyleConfig, profile.SceneRules)
	}
	if profile.PersonalityConfig["version"] != "runtime-profile-v1" {
		t.Fatalf("missing version default: %#v", profile.PersonalityConfig)
	}
}

func TestGetRuntimeProfileUsesVersionedDefaultForBrokenJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&Character{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(app.NewAppContext(db, nil))
	if err := repo.Create(&Character{
		ID:                "char-2",
		Name:              "Broken",
		PersonalityConfig: `{bad`,
		ChatStyleConfig:   "null",
		SceneRules:        "[]",
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := repo.GetRuntimeProfile("char-2")
	if err != nil {
		t.Fatal(err)
	}
	if profile.PersonalityConfig["version"] != "runtime-profile-v1" {
		t.Fatalf("expected default personality config: %#v", profile.PersonalityConfig)
	}
	if profile.ChatStyleConfig["version"] != "runtime-profile-v1" || profile.SceneRules["version"] != "runtime-profile-v1" {
		t.Fatalf("expected versioned defaults: %#v %#v", profile.ChatStyleConfig, profile.SceneRules)
	}
	if len(profile.Diagnostics) != 3 {
		t.Fatalf("expected diagnostics for broken json, got %#v", profile.Diagnostics)
	}
}
