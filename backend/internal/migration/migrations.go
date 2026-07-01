package migration

func DefaultMigrations() []Migration {
	return []Migration{
		{
			Version: "001",
			Name:    "add_psyche_states_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_states (" +
					"character_id TEXT PRIMARY KEY," +
					"version TEXT DEFAULT ''," +
					"state_version INTEGER DEFAULT 0," +
					"emotion TEXT DEFAULT '{}'," +
					"mood TEXT DEFAULT '{}'," +
					"stress REAL DEFAULT 0," +
					"energy REAL DEFAULT 0.7," +
					"created_at TEXT DEFAULT ''," +
					"updated_at TEXT DEFAULT ''" +
					")")
				return nil
			},
		},
	}
}
