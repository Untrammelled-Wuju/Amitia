package migration

func ConversationModelSettingsMigration() Migration {
	return Migration{
		Version: "20260920002",
		Name:    "add_conversation_model_settings",
		Up: func(s *Step) error {
			s.AddColumn("conversations", "model_config_id", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("conversations", "reasoning_effort", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("conversations", "reasoning_enabled", "INTEGER NOT NULL DEFAULT -1")
			return nil
		},
	}
}
