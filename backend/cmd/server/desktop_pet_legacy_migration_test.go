package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

type migrationPortStub struct {
	calls int
}

func (s *migrationPortStub) MigrateLegacyPackage(
	context.Context,
	release.LegacyPackageBuildRequest,
) (*release.LegacyPackageBuildResult, error) {
	s.calls++
	return &release.LegacyPackageBuildResult{
		PetID: "pet-migrated",
		Release: &release.ReleaseData{
			ID:              "release-migrated",
			ContentRootHash: "content-hash",
			ManifestHash:    "manifest-hash",
			Lifecycle:       string(release.ReleaseLifecycleReady),
		},
	}, nil
}

func TestLegacyDesktopPetMigrationIsIdempotentAndBindsInstallations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE desktop_pet_packages (id TEXT PRIMARY KEY, space_id TEXT, name TEXT, version INTEGER, status TEXT, package_path TEXT, manifest_path TEXT, generation_task_id TEXT, processing_task_id TEXT, created_at TEXT)`,
		`CREATE TABLE desktop_pet_legacy_package_mappings (id TEXT PRIMARY KEY, legacy_package_id TEXT, migrated_pet_id TEXT, migrated_release_id TEXT, migration_status TEXT, source_content_hash TEXT, owner_space_id TEXT, source_manifest_hash TEXT, migration_operation_id TEXT, error_message TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE desktop_pet_installations (id TEXT PRIMARY KEY, space_id TEXT, device_id TEXT, package_id TEXT, pet_id TEXT, current_release_id TEXT, legacy_package_id TEXT)`,
		`CREATE TABLE desktop_pet_runtime_desired_states (id TEXT PRIMARY KEY, space_id TEXT, device_id TEXT, installation_id TEXT, pet_id TEXT, release_id TEXT)`,
		`CREATE TABLE desktop_pet_device_active_installation_bindings (id TEXT PRIMARY KEY, space_id TEXT, device_id TEXT, installation_id TEXT, pet_id TEXT, release_id TEXT)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`INSERT INTO desktop_pet_packages (id, space_id, name, version, status, package_path, manifest_path, created_at)
VALUES ('package-old', 'space-one', '旧桌宠', 3, 'ready', 'desktop-pets/generation-tasks/task/packages/package-old', 'desktop-pets/generation-tasks/task/packages/package-old/manifest.json', '2026-10-01')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_installations (id, space_id, device_id, package_id, pet_id, current_release_id)
VALUES ('installation-old', 'space-one', 'device-one', 'package-old', '', '')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_runtime_desired_states (id, space_id, device_id, installation_id, pet_id, release_id)
VALUES ('desired-old', 'space-one', 'device-one', 'installation-old', '', '')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO desktop_pet_device_active_installation_bindings (id, space_id, device_id, installation_id, pet_id, release_id)
VALUES ('binding-old', 'space-one', 'device-one', 'installation-old', '', '')`).Error; err != nil {
		t.Fatal(err)
	}

	port := &migrationPortStub{}
	for attempt := 0; attempt < 2; attempt++ {
		if err := migrateLegacyDesktopPetPackages(t.Context(), db, port, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	if port.calls != 1 {
		t.Fatalf("migration called %d times, want 1", port.calls)
	}

	var installation legacyDesktopPetMigrationTarget
	if err := db.Table("desktop_pet_installations").
		Select("id, space_id, device_id, current_release_id").
		Where("id = ?", "installation-old").
		Scan(&installation).Error; err != nil {
		t.Fatal(err)
	}
	if installation.CurrentReleaseID != "release-migrated" {
		t.Fatalf("current release=%s", installation.CurrentReleaseID)
	}
	for _, table := range []string{"desktop_pet_runtime_desired_states", "desktop_pet_device_active_installation_bindings"} {
		var petID, releaseID string
		if err := db.Table(table).Select("pet_id, release_id").Where("installation_id = ?", "installation-old").Row().Scan(&petID, &releaseID); err != nil {
			t.Fatal(err)
		}
		if petID != "pet-migrated" || releaseID != "release-migrated" {
			t.Fatalf("%s pet=%s release=%s", table, petID, releaseID)
		}
	}
}
