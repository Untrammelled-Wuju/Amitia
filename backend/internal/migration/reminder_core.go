package migration

func ReminderCoreMigration() Migration {
	return Migration{
		Version: "20260914003",
		Name:    "restore_reminder_core_tables",
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS reminders (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id TEXT NOT NULL DEFAULT 'default',
				title TEXT DEFAULT '',
				content TEXT DEFAULT '',
				channel TEXT DEFAULT 'web',
				character_id TEXT DEFAULT '',
				conversation_id TEXT DEFAULT '',
				remind_at TEXT DEFAULT '',
				repeat_rule TEXT DEFAULT 'none',
				enabled INTEGER DEFAULT 1,
				last_triggered_at TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)
			if err := step.AddColumn("reminders", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			step.CreateTable(`CREATE TABLE IF NOT EXISTS trigger_histories (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL DEFAULT 'default',
				trigger_id TEXT DEFAULT '',
				trigger_type TEXT DEFAULT 'reminder',
				title TEXT DEFAULT '',
				channel TEXT DEFAULT 'web',
				state TEXT DEFAULT 'pending',
				priority TEXT DEFAULT 'normal',
				reason TEXT DEFAULT '',
				attempt_count INTEGER DEFAULT 0,
				last_error TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)
			step.CreateIndex("idx_reminders_user", "reminders", []string{"user_id", "enabled", "remind_at"}, false)
			step.CreateIndex("idx_trigger_histories_user", "trigger_histories", []string{"user_id", "created_at"}, false)
			return nil
		},
	}
}
