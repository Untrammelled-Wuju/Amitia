package chat

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRepositoryModelRoutesRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "routes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE model_scenario_routes (scenario TEXT PRIMARY KEY, model_config_id INTEGER NOT NULL DEFAULT 0)`).Error; err != nil {
		t.Fatal(err)
	}
	repo := &repository{db: db}
	routes, err := repo.GetModelRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("initial route count = %d, want 0", len(routes))
	}
	if err := repo.UpdateModelRoutes([]map[string]interface{}{
		{"scenario": "chat", "modelConfigId": 1},
		{"scenario": "vision", "modelConfigId": 2},
	}); err != nil {
		t.Fatal(err)
	}
	routes, err = repo.GetModelRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("route count = %d, want 2", len(routes))
	}
}
