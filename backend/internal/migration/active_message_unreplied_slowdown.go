package migration

func ActiveMessageUnrepliedSlowdownMigration() Migration {
	return Migration{
		Version: "20260904002",
		Name:    "add active message unreplied slowdown settings",
		Up: func(step *Step) error {
			if err := step.AddColumn("active_message_settings", "unreplied_slowdown_enabled", "INTEGER DEFAULT 1"); err != nil {
				return err
			}
			if err := step.AddColumn("active_message_settings", "unreplied_slowdown_after", "INTEGER DEFAULT 2"); err != nil {
				return err
			}
			if err := step.AddColumn("active_message_settings", "unreplied_cooldown_multiplier", "REAL DEFAULT 2.0"); err != nil {
				return err
			}
			return step.AddColumn("active_message_settings", "unreplied_recovery_on_reply", "INTEGER DEFAULT 1")
		},
	}
}
