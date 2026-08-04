package migration

import "testing"

func TestMigrationVersionsAreUnique(t *testing.T) {
	migrations := DefaultMigrations()
	seen := map[string]string{}

	for _, migration := range migrations {
		if previous, ok := seen[migration.Version]; ok {
			t.Fatalf(
				"duplicate migration version %s: %s and %s",
				migration.Version,
				previous,
				migration.Name,
			)
		}
		seen[migration.Version] = migration.Name
	}
}
