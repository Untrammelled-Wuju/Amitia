package migration

func SearchCredentialCleanupMigration() Migration {
	return Migration{
		Version: "20261007004",
		Name:    "search_credential_cleanup_outbox",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS search_credential_cleanup (
				secret_ref TEXT PRIMARY KEY,
				engine_id TEXT NOT NULL,
				attempts INTEGER NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL
			)`)
			return nil
		},
	}
}
