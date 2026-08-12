package migration

func WorkspaceMountsMigration() Migration {
	return Migration{
		Version: "202608120001",
		Name:    "add_workspace_mounts_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS workspace_mounts (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				kind TEXT NOT NULL,
				local_root TEXT,
				native_grant_id TEXT,
				read_only INTEGER NOT NULL DEFAULT 0,
				enabled INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`)
			return nil
		},
	}
}
