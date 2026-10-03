package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecuteDesktopPetEditingWriteCanary_UsesProductionServiceAndCleansUp(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migration.ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	if err := migration.MarkAllMigrationsApplied(db, migration.DefaultMigrations()); err != nil {
		t.Fatalf("mark migrations: %v", err)
	}
	repo := editing.NewRepository(db)
	dataDir := t.TempDir()
	svc := editing.NewService(repo, editing.NewAssetStore(dataDir, repo), nil, nil, nil, db, dataDir)
	if err := executeDesktopPetEditingWriteCanary(context.Background(), svc, db); err != nil {
		t.Fatalf("editing write canary: %v", err)
	}
	var revisions int64
	if err := db.Model(&editing.ActionRevision{}).Where("id LIKE ?", "cutover-canary-rev-%").Count(&revisions).Error; err != nil {
		t.Fatalf("count canary revisions: %v", err)
	}
	if revisions != 0 {
		t.Fatalf("expected canary prerequisite revision cleanup, got %d rows", revisions)
	}
	var sessions int64
	if err := db.Model(&editing.EditSession{}).Where("client_instance_id = ?", "migration-cutover-canary").Count(&sessions).Error; err != nil {
		t.Fatalf("count canary sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("expected canary session cleanup, got %d rows", sessions)
	}
}

func TestVerifyDesktopPetProductionReader_UsesCanonicalRepositoryV2Data(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&installation.Installation{}, &desired.RuntimeDesiredState{}, &binding.DeviceActiveInstallationBinding{}); err != nil {
		t.Fatalf("migrate reader canary schema: %v", err)
	}
	inst := &installation.Installation{ID: "inst-reader", SpaceID: "u", DeviceID: "d", PetID: "pet", CurrentReleaseID: "rel", Status: installation.StatusEnabled}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create installation: %v", err)
	}
	state := &desired.RuntimeDesiredState{ID: "ds-reader", SpaceID: "u", DeviceID: "d", RuntimeID: "r", InstallationID: inst.ID, PetID: "pet", ReleaseID: "rel", DesiredRevision: 1}
	if err := db.Create(state).Error; err != nil {
		t.Fatalf("create desired state: %v", err)
	}
	active := &binding.DeviceActiveInstallationBinding{SpaceID: "u", DeviceID: "d", InstallationID: inst.ID, PetID: "pet", ReleaseID: "rel", BindingRevision: 1}
	if err := db.Create(active).Error; err != nil {
		t.Fatalf("create binding: %v", err)
	}
	repo := installation.NewRepository(db, nil)
	if err := verifyDesktopPetProductionReader(context.Background(), repo); err != nil {
		t.Fatalf("verify canonical production reader: %v", err)
	}
}
