package migration

func ModelLocalRuntimeMigration() Migration {
	return Migration{
		Version: "202610310001",
		Name:    "add_local_model_library",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS model_artifacts (
				id TEXT PRIMARY KEY,
				owner_space_id TEXT NOT NULL DEFAULT '',
				name TEXT NOT NULL DEFAULT '',
				filename TEXT NOT NULL DEFAULT '',
				format TEXT NOT NULL DEFAULT '',
				size_bytes INTEGER NOT NULL DEFAULT 0,
				sha256 TEXT NOT NULL DEFAULT '',
				resource_uri TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'ready',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT ''
			)`)
			s.CreateIndex("idx_model_artifacts_owner_sha256", "model_artifacts", []string{"owner_space_id", "sha256"}, true)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS model_artifact_references (
				artifact_id TEXT NOT NULL DEFAULT '',
				model_config_id INTEGER NOT NULL DEFAULT 0,
				created_at TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (artifact_id, model_config_id)
			)`)
			s.CreateIndex("idx_model_artifact_refs_config", "model_artifact_references", []string{"model_config_id"}, false)
			return nil
		},
	}
}
