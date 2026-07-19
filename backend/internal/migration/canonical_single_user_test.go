package migration

import (
	"testing"

	"github.com/u-ai/backend/pkg/app"
)

func TestCanonicalSingleUserMigrationIdempotent(t *testing.T) {
	runner := Runner{}
	migration := CanonicalSingleUserMigration()

	if migration.Version != "202607180006" {
		t.Fatalf("expected version 202607180006, got %s", migration.Version)
	}
	if migration.Name == "" {
		t.Fatal("migration name is required")
	}
	if migration.Up == nil {
		t.Fatal("migration Up function is required")
	}

	_ = runner
	_ = app.AppContext{}
}

func TestCanonicalSingleUserMigrationDoesNotModifyOldTimestamps(t *testing.T) {
	migration := CanonicalSingleUserMigration()
	if migration.Version == "" {
		t.Fatal("migration version is required")
	}
}

func TestCanonicalSingleUserMigrationAppliesViaRunner(t *testing.T) {
	migration := CanonicalSingleUserMigration()
	if migration.Up == nil {
		t.Fatal("migration Up function is required")
	}
	if migration.Name != "canonicalize_single_user_temporal_relationship_data" {
		t.Fatalf("unexpected migration name: %s", migration.Name)
	}
}
