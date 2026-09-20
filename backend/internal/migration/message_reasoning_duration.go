package migration

func MessageReasoningDurationMigration() Migration {
	return Migration{
		Version: "20260920001",
		Name:    "add_message_reasoning_duration",
		Up: func(s *Step) error {
			return s.AddColumn("messages", "reasoning_duration_ms", "INTEGER NOT NULL DEFAULT 0")
		},
	}
}
