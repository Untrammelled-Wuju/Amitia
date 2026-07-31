package migration

func DesktopPetGenerationPlanTablesMigration() Migration {
	return Migration{
		Version: "202607311002",
		Name:    "add_desktop_pet_generation_plan_tables",
		Up: func(s *Step) error {
			if err := createTaskPlansTable(s); err != nil {
				return err
			}
			if err := createActionPlansTable(s); err != nil {
				return err
			}
			if err := createReferenceAssetPublishJournalsTable(s); err != nil {
				return err
			}
			if err := createGenerationOutboxTable(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func createTaskPlansTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_task_plans (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL DEFAULT '',
		schema_version INTEGER NOT NULL DEFAULT 1,
		plan_hash TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		config_id INTEGER NOT NULL DEFAULT 0,
		config_revision TEXT NOT NULL DEFAULT '',
		capability_snapshot_json TEXT NOT NULL DEFAULT '{}',
		capability_snapshot_hash TEXT NOT NULL DEFAULT '',
		reference_asset_id TEXT NOT NULL DEFAULT '',
		cost_estimate_json TEXT NOT NULL DEFAULT '{}',
		planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
		planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
		plan_json TEXT NOT NULL DEFAULT '{}',
		frozen_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_task_plans_task_id ON desktop_pet_generation_task_plans(task_id)")
	return nil
}

func createActionPlansTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_action_plans (
		id TEXT PRIMARY KEY,
		task_plan_id TEXT NOT NULL DEFAULT '',
		task_id TEXT NOT NULL DEFAULT '',
		task_action_id TEXT NOT NULL DEFAULT '',
		action_key TEXT NOT NULL DEFAULT '',
		schema_version INTEGER NOT NULL DEFAULT 1,
		plan_hash TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		config_id INTEGER NOT NULL DEFAULT 0,
		config_revision TEXT NOT NULL DEFAULT '',
		capability_hash TEXT NOT NULL DEFAULT '',
		reference_asset_id TEXT NOT NULL DEFAULT '',
		layout_json TEXT NOT NULL DEFAULT '{}',
		layout_hash TEXT NOT NULL DEFAULT '',
		prompt_snapshot TEXT NOT NULL DEFAULT '',
		prompt_hash TEXT NOT NULL DEFAULT '',
		negative_prompt_snapshot TEXT NOT NULL DEFAULT '',
		negative_prompt_hash TEXT NOT NULL DEFAULT '',
		seed_policy TEXT NOT NULL DEFAULT '',
		seed_value INTEGER,
		output_count INTEGER NOT NULL DEFAULT 1,
		target_frame_count INTEGER NOT NULL DEFAULT 0,
		planned_segment_count INTEGER NOT NULL DEFAULT 0,
		planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
		planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
		planned_call_count INTEGER NOT NULL DEFAULT 0,
		sheet_width INTEGER NOT NULL DEFAULT 0,
		sheet_height INTEGER NOT NULL DEFAULT 0,
		cell_width INTEGER NOT NULL DEFAULT 0,
		cell_height INTEGER NOT NULL DEFAULT 0,
		fallback_mode TEXT NOT NULL DEFAULT '',
		action_spec_version TEXT NOT NULL DEFAULT '',
		action_catalog_hash TEXT NOT NULL DEFAULT '',
		provider_config_hash TEXT NOT NULL DEFAULT '',
		safety_policy_version TEXT NOT NULL DEFAULT '',
		output_format TEXT NOT NULL DEFAULT '',
		plan_json TEXT NOT NULL DEFAULT '{}',
		frozen_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_action_plans_task_action_id ON desktop_pet_generation_action_plans(task_action_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_action_plans_task_plan_id ON desktop_pet_generation_action_plans(task_plan_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_action_plans_task_id ON desktop_pet_generation_action_plans(task_id)")
	return nil
}

func createReferenceAssetPublishJournalsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_reference_asset_publish_journals (
		id TEXT PRIMARY KEY,
		reference_asset_id TEXT NOT NULL DEFAULT '',
		staging_path TEXT NOT NULL DEFAULT '',
		final_path TEXT NOT NULL DEFAULT '',
		source_storage_key TEXT NOT NULL DEFAULT '',
		normalized_storage_key TEXT NOT NULL DEFAULT '',
		content_hash TEXT NOT NULL DEFAULT '',
		journal_status TEXT NOT NULL DEFAULT 'staging',
		error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_ref_asset_publish_journals_reference_asset_id ON desktop_pet_reference_asset_publish_journals(reference_asset_id)")
	return nil
}

func createGenerationOutboxTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_outbox (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL DEFAULT '',
		task_action_id TEXT NOT NULL DEFAULT '',
		attempt_id TEXT NOT NULL DEFAULT '',
		event_type TEXT NOT NULL DEFAULT '',
		payload_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'pending',
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3,
		next_retry_at TEXT NOT NULL DEFAULT '',
		processed_at TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_generation_outbox_status ON desktop_pet_generation_outbox(status)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_generation_outbox_task_action_id ON desktop_pet_generation_outbox(task_action_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_generation_outbox_attempt_id ON desktop_pet_generation_outbox(attempt_id)")
	return nil
}
