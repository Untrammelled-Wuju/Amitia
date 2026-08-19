package migration

func SurrealSchemaMigration() Migration {
	return Migration{
		Version:           "surreal:001",
		Name:              "surreal_schema_baseline",
		AcceptedChecksums: []string{"84d4ae791b018556390f61a7af448dcd7532fe0e41621a9b8469c8343cc81918"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS surreal_schema_versions (
				schema_version TEXT PRIMARY KEY,
				entity_types TEXT NOT NULL DEFAULT '',
				edge_types TEXT NOT NULL DEFAULT '',
				created_at TEXT DEFAULT ''
			)`)
			return nil
		},
	}
}
