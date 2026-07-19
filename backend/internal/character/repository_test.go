package character

import (
	"encoding/json"
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
		CharacterBase:     "保持角色一致",
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

func TestCreateThenUpdatePersonalityConfigRoundTrip(t *testing.T) {
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
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc := NewService(repo, ctx)

	created, err := svc.Create(&CreateCharacterRequest{
		Name:              "测试角色",
		Identity:          "一个测试角色",
		PersonalityConfig: json.RawMessage(`{"warmth":80,"sensitivity":60,"tolerance":70}`),
		ChatStyleConfig:   `{"pace":"fast","emotion":"rich"}`,
		SceneRules:        `{"place":"office","time":"day"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.PersonalityConfig != `{"warmth":80,"sensitivity":60,"tolerance":70}` {
		t.Fatalf("personalityConfig not saved: %s", created.PersonalityConfig)
	}
	if created.ChatStyleConfig != `{"pace":"fast","emotion":"rich"}` {
		t.Fatalf("chatStyleConfig not saved: %s", created.ChatStyleConfig)
	}
	if created.SceneRules != `{"place":"office","time":"day"}` {
		t.Fatalf("sceneRules not saved: %s", created.SceneRules)
	}

	read, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.PersonalityConfig != `{"warmth":80,"sensitivity":60,"tolerance":70}` {
		t.Fatalf("personalityConfig read mismatch: %s", read.PersonalityConfig)
	}
	if read.ChatStyleConfig != `{"pace":"fast","emotion":"rich"}` {
		t.Fatalf("chatStyleConfig read mismatch: %s", read.ChatStyleConfig)
	}
	if read.SceneRules != `{"place":"office","time":"day"}` {
		t.Fatalf("sceneRules read mismatch: %s", read.SceneRules)
	}

	newConfig := json.RawMessage(`{"warmth":80,"sensitivity":60,"tolerance":30}`)
	cfg := newConfig
	updated, err := svc.Update(created.ID, &UpdateCharacterRequest{
		PersonalityConfig: &cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.PersonalityConfig != string(newConfig) {
		t.Fatalf("personalityConfig update not persisted: %s", updated.PersonalityConfig)
	}
	if updated.ChatStyleConfig != `{"pace":"fast","emotion":"rich"}` {
		t.Fatalf("chatStyleConfig should not change: %s", updated.ChatStyleConfig)
	}
	if updated.SceneRules != `{"place":"office","time":"day"}` {
		t.Fatalf("sceneRules should not change: %s", updated.SceneRules)
	}

	read2, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read2.PersonalityConfig != string(newConfig) {
		t.Fatalf("re-read personalityConfig mismatch: %s", read2.PersonalityConfig)
	}

	profile, err := repo.GetRuntimeProfile(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.PersonalityConfig["warmth"].(float64) != 80 {
		t.Fatalf("runtime warmth mismatch: %v", profile.PersonalityConfig["warmth"])
	}
	if profile.PersonalityConfig["tolerance"].(float64) != 30 {
		t.Fatalf("runtime tolerance mismatch: %v", profile.PersonalityConfig["tolerance"])
	}
	if profile.ChatStyleConfig["pace"] != "fast" {
		t.Fatalf("runtime chatStyleConfig mismatch: %v", profile.ChatStyleConfig["pace"])
	}
	if profile.SceneRules["place"] != "office" {
		t.Fatalf("runtime sceneRules mismatch: %v", profile.SceneRules["place"])
	}
}
