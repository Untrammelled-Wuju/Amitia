package migration

func CharacterModelSettingsMigration() Migration {
	return Migration{
		Version: "20261103002",
		Name:    "add_character_model_settings",
		Up: func(s *Step) error {
			if err := s.AddColumn("characters", "model_config_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("characters", "reasoning_effort", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return s.AddColumn("characters", "reasoning_enabled", "INTEGER NOT NULL DEFAULT -1")
		},
	}
}
