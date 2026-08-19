package migration

func NeedStatesMigration() Migration {
	return Migration{
		Version:           "202607090001",
		Name:              "add_need_states_model_configs_message_count",
		AcceptedChecksums: []string{"399b307e5d9dd6a7a7a9899e49cbd2b4e6eae2ac99e12719cd29e5066ed70524"},
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS need_states (
				id TEXT PRIMARY KEY,
				character_id TEXT NOT NULL DEFAULT '',
				need_key TEXT NOT NULL DEFAULT '',
				current_value REAL DEFAULT 0,
				baseline REAL DEFAULT 0,
				trend REAL DEFAULT 0,
				saturated INTEGER DEFAULT 0,
				updated_at TEXT DEFAULT ''
			)`)
			if err := step.CreateIndex("idx_need_states_char_key", "need_states", []string{"character_id", "need_key"}, true); err != nil {
				return err
			}

			step.CreateTable(`CREATE TABLE IF NOT EXISTS model_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT DEFAULT '',
				api_type TEXT DEFAULT '',
				base_url TEXT DEFAULT '',
				api_key TEXT DEFAULT '',
				model_name TEXT DEFAULT '',
				temperature REAL DEFAULT 0.7,
				max_tokens INTEGER DEFAULT 4096,
				top_p REAL DEFAULT 1,
				timeout_seconds INTEGER DEFAULT 60,
				retry_count INTEGER DEFAULT 1,
				is_active INTEGER DEFAULT 0,
				last_test_status TEXT DEFAULT '',
				last_test_message TEXT DEFAULT '',
				last_test_at TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)

			return step.AddColumn("conversations", "message_count", "INTEGER DEFAULT 0")
		},
	}
}
