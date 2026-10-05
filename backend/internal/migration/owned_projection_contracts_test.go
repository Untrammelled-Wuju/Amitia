package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOwnedProjectionContractsBaselineAndUpgrade(t *testing.T) {
	for _, fresh := range []bool{true, false} {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "projection.db")), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		t.Cleanup(func() { _ = sqlDB.Close() })
		if fresh {
			if err := ApplyBaseline(db); err != nil {
				t.Fatal(err)
			}
		} else {
			for _, query := range []string{
				`CREATE TABLE qdrant_collection_versions(collection_name TEXT PRIMARY KEY,vector_dim INTEGER,distance TEXT,created_at TEXT)`,
				`CREATE TABLE surreal_schema_versions(schema_version TEXT PRIMARY KEY,entity_types TEXT,edge_types TEXT,created_at TEXT)`,
				`INSERT INTO qdrant_collection_versions VALUES('existing',2,'Cosine','old')`,
				`INSERT INTO surreal_schema_versions VALUES('existing','node','edge','old')`,
			} {
				if err := db.Exec(query).Error; err != nil {
					t.Fatal(err)
				}
			}
			for _, migration := range []Migration{QdrantOwnedProjectionMigration(), SurrealOwnedProjectionMigration()} {
				step := &Step{db: db}
				if err := migration.Up(step); err != nil {
					t.Fatal(err)
				}
				for _, command := range step.commands {
					if err := db.Exec(command).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := migration.Up(&Step{db: db}); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !db.Migrator().HasColumn("qdrant_collection_versions", "schema_version") || !db.Migrator().HasColumn("surreal_schema_versions", "projection_contract") {
			t.Fatal("projection contract columns missing")
		}
		if !fresh {
			var count int64
			db.Table("qdrant_collection_versions").Where("collection_name = ?", "existing").Count(&count)
			if count != 1 {
				t.Fatal("legacy collection metadata lost")
			}
		}
	}
	versions := map[string]bool{}
	for _, migration := range DefaultMigrations() {
		versions[migration.Version] = true
	}
	if !versions["qdrant:002"] || !versions["surreal:002"] {
		t.Fatal("projection migrations unregistered")
	}
}
