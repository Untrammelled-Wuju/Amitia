package migration

func ContinuityRuntimeCompletionMigration() Migration {
	return Migration{
		Version: "20260922002",
		Name:    "continuity_runtime_completion",
		Up: func(s *Step) error {
			columns := []struct {
				name     string
				typeDecl string
			}{
				{"resolved_by", "TEXT NOT NULL DEFAULT ''"},
				{"resolution_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"auto_resume", "INTEGER NOT NULL DEFAULT 1"},
				{"wake_state", "TEXT NOT NULL DEFAULT ''"},
				{"wake_request_id", "TEXT NOT NULL DEFAULT ''"},
				{"wake_attempts", "INTEGER NOT NULL DEFAULT 0"},
				{"next_wake_at", "DATETIME"},
				{"last_wake_error", "TEXT NOT NULL DEFAULT ''"},
				{"wake_delivered_at", "DATETIME"},
			}
			for _, column := range columns {
				if err := s.AddColumn("continuity_waits", column.name, column.typeDecl); err != nil {
					return err
				}
			}
			// V1 only created conversational/user waits. Those must stay inline-only:
			// resolving the current user message must not emit a second proactive turn.
			s.Execute("UPDATE continuity_waits SET auto_resume = 0 WHERE wait_type = 'user'")
			if err := s.CreateIndex("idx_continuity_waits_wake_due", "continuity_waits", []string{"wake_state", "next_wake_at"}, false); err != nil {
				return err
			}
			return s.CreateIndex("idx_continuity_waits_due_status", "continuity_waits", []string{"wait_type", "status", "due_at"}, false)
		},
	}
}
