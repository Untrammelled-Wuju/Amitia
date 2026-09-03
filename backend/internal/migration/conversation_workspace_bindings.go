package migration

func ConversationWorkspaceBindingsMigration() Migration {
	return Migration{
		Version: "20260903002",
		Name:    "add_conversation_workspace_bindings",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS conversation_workspace_bindings (
				conversation_id TEXT PRIMARY KEY,
				workspace_id TEXT NOT NULL,
				device_id TEXT NOT NULL DEFAULT '',
				workspace_name TEXT NOT NULL DEFAULT '',
				workspace_kind TEXT NOT NULL DEFAULT 'local',
				root_uri TEXT NOT NULL DEFAULT '',
				updated_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_conversation_workspace_bindings_workspace", "conversation_workspace_bindings", []string{"workspace_id"}, false)
			s.CreateIndex("idx_conversation_workspace_bindings_device", "conversation_workspace_bindings", []string{"device_id"}, false)
			return nil
		},
	}
}
