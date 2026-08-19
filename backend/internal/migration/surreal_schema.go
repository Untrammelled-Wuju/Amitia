package migration

func SurrealSchemaMigration() Migration {
	return Migration{
		Version: "surreal:001",
		Name:    "surreal_schema_baseline",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS surreal_schema_versions (
				schema_version TEXT PRIMARY KEY,
				entity_types TEXT NOT NULL DEFAULT '',
				edge_types TEXT NOT NULL DEFAULT '',
				created_at TEXT DEFAULT (datetime('now'))
			)`)
			return nil
		},
	}
}
