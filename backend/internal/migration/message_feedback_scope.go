package migration

func MessageFeedbackScopeMigration() Migration {
	return Migration{
		Version: "202610230001",
		Name:    "scope_message_feedback",
		Up: func(step *Step) error {
			columns := []struct {
				name       string
				definition string
			}{
				{"space_id", "TEXT NOT NULL DEFAULT ''"},
				{"owner_id", "TEXT NOT NULL DEFAULT ''"},
				{"core_id", "TEXT NOT NULL DEFAULT ''"},
				{"role_id", "TEXT NOT NULL DEFAULT ''"},
				{"conversation_id", "TEXT NOT NULL DEFAULT ''"},
				{"request_id", "TEXT NOT NULL DEFAULT ''"},
				{"revision", "INTEGER NOT NULL DEFAULT 1"},
				{"updated_at", "TEXT DEFAULT ''"},
				{"deleted_at", "DATETIME"},
			}
			for _, column := range columns {
				if err := step.AddColumn("message_feedback", column.name, column.definition); err != nil {
					return err
				}
			}
			indexes := []struct {
				name    string
				columns []string
			}{
				{"idx_message_feedback_scope", []string{"space_id", "owner_id", "core_id", "role_id"}},
				{"idx_message_feedback_message", []string{"message_id"}},
				{"idx_message_feedback_request", []string{"request_id"}},
				{"idx_message_feedback_deleted_at", []string{"deleted_at"}},
			}
			for _, index := range indexes {
				if err := step.CreateIndex(index.name, "message_feedback", index.columns, false); err != nil {
					return err
				}
			}
			step.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_message_feedback_scoped_request ON message_feedback(space_id, owner_id, core_id, role_id, request_id) WHERE request_id <> ''")
			return nil
		},
	}
}
