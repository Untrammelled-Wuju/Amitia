package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestModelScenarioRoutesMigrationIsRegisteredAndApplied(t *testing.T) {
	registered := false
	for _, item := range DefaultMigrations() {
		if item.Version == "20260921002" {
			registered = item.Name == "create_model_scenario_routes"
			break
		}
	}
	if !registered {
		t.Fatal("model scenario routes migration is not registered")
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "routes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='model_scenario_routes'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("model_scenario_routes table count = %d, want 1", count)
	}
}
