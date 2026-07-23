package system

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSetupServiceTest(t *testing.T) *service {
	t.Helper()
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
	if err := db.Exec(`CREATE TABLE app_settings (
		key TEXT PRIMARY KEY,
		value TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return &service{db: db}
}

func TestSetupAndOnboardingStateCompatibility(t *testing.T) {
	svc := newSetupServiceTest(t)

	svc.setAppSetting("setup_completed", "true")
	if svc.OnboardingStatus()["completed"] != true {
		t.Fatal("legacy setup state was not recognized")
	}

	svc.OnboardingReset()
	svc.OnboardingComplete()
	if svc.SetupStatus()["completed"] != true {
		t.Fatal("onboarding completion was not synchronized")
	}
	if svc.getAppSetting("setup_step") != "done" {
		t.Fatal("setup step was not synchronized")
	}

	svc.SetupReset()
	if svc.OnboardingStatus()["completed"] != false {
		t.Fatal("reset state was not synchronized")
	}
}
