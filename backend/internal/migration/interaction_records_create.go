package migration

func InteractionRecordsCreateMigration() Migration {
	return Migration{
		Version: "202607030000",
		Name:    "create_interaction_records_table",
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS interaction_records (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				character_id TEXT,
				conversation_id TEXT,
				channel TEXT,
				peer_id TEXT,
				session_id TEXT,
				source TEXT,
				request_id TEXT,
				priority INTEGER,
				path_type TEXT,
				status TEXT,
				status_version INTEGER NOT NULL DEFAULT 0,
				supersedes_id TEXT,
				superseded_by_id TEXT,
				cancel_reason TEXT,
				error_code TEXT,
				error_message TEXT,
				result_ref TEXT,
				commit_id TEXT,
				executor_id TEXT,
				deadline_at DATETIME,
				cancel_requested_at DATETIME,
				created_at DATETIME,
				started_at DATETIME,
				committed_at DATETIME,
				completed_at DATETIME,
				updated_at DATETIME
			)`)

			if err := step.CreateIndex("idx_interaction_scope_active", "interaction_records", []string{"user_id", "character_id", "conversation_id", "status"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_request", "interaction_records", []string{"user_id", "request_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_channel", "interaction_records", []string{"channel"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_peer_id", "interaction_records", []string{"peer_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_session_id", "interaction_records", []string{"session_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_priority", "interaction_records", []string{"priority"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_supersedes_id", "interaction_records", []string{"supersedes_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_superseded_by_id", "interaction_records", []string{"superseded_by_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_executor_id", "interaction_records", []string{"executor_id"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_created_at", "interaction_records", []string{"created_at"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_records_updated_at", "interaction_records", []string{"updated_at"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_interaction_request_unique", "interaction_records", []string{"user_id", "request_id"}, true); err != nil {
				return err
			}

			return nil
		},
	}
}
