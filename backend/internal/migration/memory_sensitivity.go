package migration

func MemorySensitivityMigration() Migration {
	return Migration{
		Version: "202607010069",
		Name:    "add memory sensitivity and proactive mention",
		Up: func(step *Step) error {
			if err := step.AddColumn("memories", "sensitivity_level", "TEXT DEFAULT 'internal'"); err != nil {
				return err
			}
			if err := step.AddColumn("memories", "allow_proactive_mention", "INTEGER DEFAULT 1"); err != nil {
				return err
			}
			if err := step.AddColumn("memories", "requires_confirmation", "INTEGER DEFAULT 0"); err != nil {
				return err
			}
			if err := step.AddColumn("memory_candidates", "sensitivity_level", "TEXT DEFAULT 'internal'"); err != nil {
				return err
			}
			if err := step.AddColumn("memory_candidates", "allow_proactive_mention", "INTEGER DEFAULT 1"); err != nil {
				return err
			}
			if err := step.AddColumn("memory_candidates", "requires_confirmation", "INTEGER DEFAULT 0"); err != nil {
				return err
			}
			return nil
		},
	}
}
