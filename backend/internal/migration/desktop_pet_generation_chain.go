package migration

func DesktopPetGenerationChainMigration() Migration {
	return Migration{
		Version: "202607310005",
		Name:    "add_desktop_pet_generation_chain_tables",
		Up: func(s *Step) error {
			if err := createActionGenerationAttemptsTable(s); err != nil {
				return err
			}
			if err := createGenerationArtifactsTable(s); err != nil {
				return err
			}
			if err := createReferenceAssetsTable(s); err != nil {
				return err
			}
			if err := addGenerationTaskPlanColumns(s); err != nil {
				return err
			}
			if err := addGenerationTaskActionPlanColumns(s); err != nil {
				return err
			}
			if err := extendGenerationCallLogColumns(s); err != nil {
				return err
			}
			backfillLegacyGenerationMode(s)
			return nil
		},
	}
}

func createActionGenerationAttemptsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_generation_attempts (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL DEFAULT '',
		task_action_id TEXT NOT NULL DEFAULT '',
		attempt_number INTEGER NOT NULL DEFAULT 1,
		parent_attempt_id TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT 'sprite_sheet',
		reason TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		config_id INTEGER NOT NULL DEFAULT 0,
		config_revision TEXT NOT NULL DEFAULT '',
		capability_hash TEXT NOT NULL DEFAULT '',
		reference_asset_id TEXT NOT NULL DEFAULT '',
		plan_json TEXT NOT NULL DEFAULT '{}',
		prompt_document_json TEXT NOT NULL DEFAULT '{}',
		prompt_snapshot TEXT NOT NULL DEFAULT '',
		prompt_hash TEXT NOT NULL DEFAULT '',
		negative_prompt_snapshot TEXT NOT NULL DEFAULT '',
		seed_policy TEXT NOT NULL DEFAULT 'auto',
		seed_value INTEGER,
		output_count INTEGER NOT NULL DEFAULT 1,
		execution_id TEXT NOT NULL DEFAULT '',
		worker_id TEXT NOT NULL DEFAULT '',
		lease TEXT NOT NULL DEFAULT '',
		provider_request_id TEXT NOT NULL DEFAULT '',
		provider_operation_id TEXT NOT NULL DEFAULT '',
		submitted_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT '',
		error_code TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE(task_action_id, attempt_number)
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_attempts_task_id ON desktop_pet_action_generation_attempts(task_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_attempts_action_id ON desktop_pet_action_generation_attempts(task_action_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_attempts_status ON desktop_pet_action_generation_attempts(status)")
	return nil
}

func createGenerationArtifactsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_artifacts (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL DEFAULT '',
		task_action_id TEXT NOT NULL DEFAULT '',
		attempt_id TEXT NOT NULL DEFAULT '',
		artifact_type TEXT NOT NULL DEFAULT 'sprite_sheet_raw',
		segment_index INTEGER NOT NULL DEFAULT 0,
		candidate_index INTEGER NOT NULL DEFAULT 0,
		is_primary INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		relative_path TEXT NOT NULL DEFAULT '',
		mime TEXT NOT NULL DEFAULT '',
		width INTEGER NOT NULL DEFAULT 0,
		height INTEGER NOT NULL DEFAULT 0,
		size INTEGER NOT NULL DEFAULT 0,
		hash TEXT NOT NULL DEFAULT '',
		provider_request_id TEXT NOT NULL DEFAULT '',
		provider_operation_id TEXT NOT NULL DEFAULT '',
		layout_json TEXT NOT NULL DEFAULT '{}',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE(attempt_id, artifact_type, segment_index, candidate_index)
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_artifacts_task_id ON desktop_pet_generation_artifacts(task_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_artifacts_attempt_id ON desktop_pet_generation_artifacts(attempt_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_gen_artifacts_action_id ON desktop_pet_generation_artifacts(task_action_id)")
	return nil
}

func createReferenceAssetsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_reference_assets (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL DEFAULT '',
		source_path TEXT NOT NULL DEFAULT '',
		source_hash TEXT NOT NULL DEFAULT '',
		source_mime TEXT NOT NULL DEFAULT '',
		source_width INTEGER NOT NULL DEFAULT 0,
		source_height INTEGER NOT NULL DEFAULT 0,
		normalized_path TEXT NOT NULL DEFAULT '',
		normalized_hash TEXT NOT NULL DEFAULT '',
		normalized_mime TEXT NOT NULL DEFAULT '',
		normalized_width INTEGER NOT NULL DEFAULT 0,
		normalized_height INTEGER NOT NULL DEFAULT 0,
		config_hash TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_ref_assets_task_id ON desktop_pet_reference_assets(task_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_ref_assets_source_hash ON desktop_pet_reference_assets(source_hash)")
	return nil
}

func addGenerationTaskPlanColumns(s *Step) error {
	cols := [][2]string{
		{"reference_asset_id", "TEXT NOT NULL DEFAULT ''"},
		{"generation_plan_version", "INTEGER NOT NULL DEFAULT 0"},
		{"provider_key_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"model_name_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"config_revision_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"capability_snapshot_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"capability_snapshot_hash", "TEXT NOT NULL DEFAULT ''"},
		{"cost_estimate_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"planned_primary_request_count", "INTEGER NOT NULL DEFAULT 0"},
		{"planned_max_provider_call_count", "INTEGER NOT NULL DEFAULT 0"},
		{"actual_provider_call_count", "INTEGER NOT NULL DEFAULT 0"},
		{"actual_output_image_count", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, col := range cols {
		if err := s.AddColumn("desktop_pet_generation_tasks", col[0], col[1]); err != nil {
			return err
		}
	}
	return nil
}

func addGenerationTaskActionPlanColumns(s *Step) error {
	cols := [][2]string{
		{"generation_mode", "TEXT NOT NULL DEFAULT 'legacy_frame'"},
		{"generation_plan_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"generation_plan_hash", "TEXT NOT NULL DEFAULT ''"},
		{"prompt_template_version", "TEXT NOT NULL DEFAULT ''"},
		{"active_attempt_id", "TEXT NOT NULL DEFAULT ''"},
		{"active_attempt_number", "INTEGER NOT NULL DEFAULT 0"},
		{"planned_segment_count", "INTEGER NOT NULL DEFAULT 0"},
		{"planned_primary_request_count", "INTEGER NOT NULL DEFAULT 0"},
		{"planned_max_provider_call_count", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, col := range cols {
		if err := s.AddColumn("desktop_pet_generation_task_actions", col[0], col[1]); err != nil {
			return err
		}
	}
	return nil
}

func extendGenerationCallLogColumns(s *Step) error {
	cols := [][2]string{
		{"attempt_id", "TEXT NOT NULL DEFAULT ''"},
		{"artifact_id", "TEXT NOT NULL DEFAULT ''"},
		{"call_type", "TEXT NOT NULL DEFAULT 'primary'"},
		{"call_attempt_index", "INTEGER NOT NULL DEFAULT 0"},
		{"idempotency_key_hash", "TEXT NOT NULL DEFAULT ''"},
		{"request_hash", "TEXT NOT NULL DEFAULT ''"},
		{"submission_state", "TEXT NOT NULL DEFAULT ''"},
		{"retry_class", "TEXT NOT NULL DEFAULT ''"},
		{"http_status", "INTEGER NOT NULL DEFAULT 0"},
		{"usage_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"estimated_cost_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"actual_cost_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"response_receipt_json", "TEXT NOT NULL DEFAULT '{}'"},
	}
	for _, col := range cols {
		if err := s.AddColumn("desktop_pet_generation_call_logs", col[0], col[1]); err != nil {
			return err
		}
	}
	return nil
}

func backfillLegacyGenerationMode(s *Step) {
	s.Execute("UPDATE desktop_pet_generation_task_actions SET generation_mode = 'legacy_frame' WHERE generation_mode = '' OR generation_mode IS NULL")
}
