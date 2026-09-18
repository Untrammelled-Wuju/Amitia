package migration

func MessageExtensionTypeMigration() Migration {
	return Migration{
		Version: "20260914001",
		Name:    "add_message_extension_type",
		Up: func(s *Step) error {
			return s.AddColumn("messages", "extension_type", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
