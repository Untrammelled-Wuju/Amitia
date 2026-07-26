package migration

func EpisodicMessageTimeMigration() Migration {
	return Migration{
		Version: "202607260001",
		Name:    "add_episodic_message_time_columns",
		Up: func(step *Step) error {
			if err := step.AddColumn("episodic_memories", "message_time_start", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return step.AddColumn("episodic_memories", "message_time_end", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
