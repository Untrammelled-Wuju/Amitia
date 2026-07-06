package migration

func TriggerHistoryMigration() Migration {
	return Migration{
		Version: "006",
		Name:    "add_trigger_histories_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS trigger_histories (
				id TEXT PRIMARY KEY,
				trigger_id TEXT NOT NULL DEFAULT '',
				trigger_type TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL DEFAULT '',
				channel TEXT NOT NULL DEFAULT 'web',
				state TEXT NOT NULL DEFAULT 'pending',
				priority TEXT NOT NULL DEFAULT 'normal',
				reason TEXT NOT NULL DEFAULT '',
				attempt_count INTEGER DEFAULT 0,
				last_error TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)
			s.CreateIndex("idx_trigger_histories_state", "trigger_histories", []string{"state"}, false)
			s.CreateIndex("idx_trigger_histories_created_at", "trigger_histories", []string{"created_at"}, false)
			return nil
		},
	}
}
