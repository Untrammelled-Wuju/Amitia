package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func TestNewAppServicesBuildsCoreServicesOnce(t *testing.T) {
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
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	if err := (migration.Runner{DB: db, SkipBackup: true}).Apply(migration.DefaultMigrations()); err != nil {
		t.Fatal(err)
	}
	services, err := NewAppServices(app.NewAppContext(db, nil), nil, nil)
	if err != nil {
		t.Fatalf("new app services: %v", err)
	}
	t.Cleanup(func() {
		_ = services.Extension.Close(context.Background())
	})
	if services.Chat == nil {
		t.Fatal("missing chat service")
	}
	if services.Memory == nil {
		t.Fatal("missing memory service")
	}
	if services.Profile == nil {
		t.Fatal("missing profile service")
	}
	if services.Episodic == nil {
		t.Fatal("missing episodic service")
	}
	if services.WorldBook == nil {
		t.Fatal("missing worldbook service")
	}
	if services.Companion == nil {
		t.Fatal("missing companion service")
	}
}
