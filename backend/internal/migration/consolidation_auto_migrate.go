package migration

func ConsolidationAutoMigrateMigration() Migration {
	return Migration{
		Version: "202607290001",
		Name:    "consolidate_auto_migrate_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS outbox_records (
				id TEXT PRIMARY KEY,
				aggregate_id TEXT DEFAULT '',
				event_type TEXT DEFAULT '',
				payload TEXT DEFAULT '',
				payload_version TEXT DEFAULT '',
				status TEXT DEFAULT '',
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until TEXT DEFAULT '',
				available_at TEXT DEFAULT '',
				published_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT '',
				retry_count INTEGER DEFAULT 0,
				max_retries INTEGER DEFAULT 0,
				next_retry_at TEXT DEFAULT '',
				last_error TEXT DEFAULT '',
				idempotency_key TEXT DEFAULT '',
				created_at TEXT DEFAULT ''
			)`)
			s.CreateIndex("idx_outbox_records_status_available", "outbox_records", []string{"status", "available_at"}, false)
			s.CreateIndex("idx_outbox_records_lease_owner", "outbox_records", []string{"lease_owner"}, false)
			s.CreateIndex("idx_outbox_records_aggregate_id", "outbox_records", []string{"aggregate_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS dead_letter_records (
				id TEXT PRIMARY KEY,
				outbox_id TEXT UNIQUE,
				event_type TEXT DEFAULT '',
				payload TEXT DEFAULT '',
				status TEXT DEFAULT '',
				retry_count INTEGER DEFAULT 0,
				max_retries INTEGER DEFAULT 0,
				next_retry_at TEXT DEFAULT '',
				last_error TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS deletion_tombstones (
				id TEXT PRIMARY KEY,
				target_id TEXT DEFAULT '',
				target_type TEXT DEFAULT '',
				scope TEXT DEFAULT '',
				status TEXT DEFAULT '',
				items_count INTEGER DEFAULT 0,
				cleaned_count INTEGER DEFAULT 0,
				failed_count INTEGER DEFAULT 0,
				requested_at DATETIME,
				blocked_until DATETIME,
				completed_at DATETIME,
				retrieval_blocked INTEGER DEFAULT 0
			)`)
			s.CreateIndex("idx_deletion_tombstones_target_id", "deletion_tombstones", []string{"target_id"}, false)
			s.CreateIndex("idx_deletion_tombstones_status", "deletion_tombstones", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS data_lifecycle_outbox_cleanup_items (
				id TEXT PRIMARY KEY,
				storage TEXT DEFAULT '',
				target_id TEXT DEFAULT '',
				target_kind TEXT DEFAULT '',
				status TEXT DEFAULT '',
				attempts INTEGER DEFAULT 0,
				max_attempts INTEGER DEFAULT 5,
				next_retry_at DATETIME,
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until DATETIME,
				last_error TEXT DEFAULT '',
				cleaned_at DATETIME
			)`)
			s.CreateIndex("idx_dl_outbox_cleanup_storage", "data_lifecycle_outbox_cleanup_items", []string{"storage"}, false)
			s.CreateIndex("idx_dl_outbox_cleanup_target_id", "data_lifecycle_outbox_cleanup_items", []string{"target_id"}, false)
			s.CreateIndex("idx_dl_outbox_cleanup_status", "data_lifecycle_outbox_cleanup_items", []string{"status"}, false)
			s.CreateIndex("idx_dl_outbox_cleanup_leased_until", "data_lifecycle_outbox_cleanup_items", []string{"leased_until"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS data_lifecycle_recalculation_tasks (
				id TEXT PRIMARY KEY,
				trigger_type TEXT DEFAULT '',
				target_id TEXT DEFAULT '',
				affected_zone TEXT DEFAULT '',
				priority INTEGER DEFAULT 0,
				created_at DATETIME,
				status TEXT DEFAULT '',
				description TEXT DEFAULT '',
				attempts INTEGER DEFAULT 0,
				max_attempts INTEGER DEFAULT 3,
				next_retry_at DATETIME,
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until DATETIME,
				last_error TEXT DEFAULT '',
				completed_at DATETIME
			)`)
			s.CreateIndex("idx_dl_recalc_trigger_type", "data_lifecycle_recalculation_tasks", []string{"trigger_type"}, false)
			s.CreateIndex("idx_dl_recalc_target_id", "data_lifecycle_recalculation_tasks", []string{"target_id"}, false)
			s.CreateIndex("idx_dl_recalc_status", "data_lifecycle_recalculation_tasks", []string{"status"}, false)
			s.CreateIndex("idx_dl_recalc_leased_until", "data_lifecycle_recalculation_tasks", []string{"leased_until"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS output_leases (
				id TEXT PRIMARY KEY,
				interaction_id TEXT DEFAULT '',
				character_id TEXT DEFAULT '',
				user_id TEXT DEFAULT '',
				channel TEXT DEFAULT '',
				owner_token TEXT DEFAULT '',
				generation INTEGER DEFAULT 0,
				status TEXT DEFAULT '',
				acquired_at TEXT DEFAULT '',
				expires_at TEXT DEFAULT '',
				released_at TEXT DEFAULT '',
				preempted_by TEXT DEFAULT ''
			)`)
			s.CreateIndex("idx_output_leases_interaction_id", "output_leases", []string{"interaction_id"}, false)
			s.CreateIndex("idx_output_leases_character_id", "output_leases", []string{"character_id"}, false)

			return nil
		},
	}
}
