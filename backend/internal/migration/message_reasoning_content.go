package migration

func MessageReasoningContentMigration() Migration {
	return Migration{
		Version: "20260919003",
		Name:    "add_message_reasoning_content",
		Up: func(s *Step) error {
			return s.AddColumn("messages", "reasoning_content", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
