package migration

func ConversationPermissionModeMigration() Migration {
	return Migration{
		Version: "20260920005",
		Name:    "add_conversation_permission_mode",
		Up: func(s *Step) error {
			s.AddColumn("conversations", "permission_mode", "TEXT NOT NULL DEFAULT 'request_approval'")
			return nil
		},
	}
}
