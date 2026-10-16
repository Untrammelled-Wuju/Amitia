package migration

func UserFeedbackTicketsMigration() Migration {
	return Migration{
		Version: "20261016001",
		Name:    "create_user_feedback_tickets_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS user_feedback_tickets (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL DEFAULT '',
				category TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL DEFAULT '',
				description TEXT NOT NULL DEFAULT '',
				contact TEXT NOT NULL DEFAULT '',
				platform TEXT NOT NULL DEFAULT '',
				client_type TEXT NOT NULL DEFAULT '',
				app_version TEXT NOT NULL DEFAULT '',
				runtime_mode TEXT NOT NULL DEFAULT '',
				os_info TEXT NOT NULL DEFAULT '',
				device_hash TEXT NOT NULL DEFAULT '',
				locale TEXT NOT NULL DEFAULT '',
				timezone TEXT NOT NULL DEFAULT '',
				include_logs INTEGER NOT NULL DEFAULT 1,
				log_line_count INTEGER NOT NULL DEFAULT 0,
				diagnostics TEXT NOT NULL DEFAULT '',
				upload_url TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'pending',
				attempts INTEGER NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				remote_id TEXT NOT NULL DEFAULT '',
				remote_url TEXT NOT NULL DEFAULT '',
				screenshot_name TEXT NOT NULL DEFAULT '',
				screenshot_path TEXT NOT NULL DEFAULT '',
				screenshot_size INTEGER NOT NULL DEFAULT 0,
				has_screenshot INTEGER NOT NULL DEFAULT 0,
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				uploaded_at TEXT NOT NULL DEFAULT ''
			)`)
			s.CreateIndex("idx_user_feedback_tickets_space", "user_feedback_tickets", []string{"space_id", "created_at"}, false)
			s.CreateIndex("idx_user_feedback_tickets_status", "user_feedback_tickets", []string{"status", "attempts"}, false)
			return nil
		},
	}
}
