package migration

func DesktopPetGenerationActiveArtifactMigration() Migration {
	return Migration{
		Version: "202607310011",
		Name:    "add_desktop_pet_generation_active_artifact_tables",
		Up: func(s *Step) error {
			if err := createActiveBindingsTable(s); err != nil {
				return err
			}
			if err := createProviderReceiptsTable(s); err != nil {
				return err
			}
			if err := createArtifactPublishJournalTable(s); err != nil {
				return err
			}
			if err := extendReferenceAssetsTable(s); err != nil {
				return err
			}
			if err := extendGenerationTaskActionsTable(s); err != nil {
				return err
			}
			if err := extendGenerationAttemptsTable(s); err != nil {
				return err
			}
			if err := extendGenerationArtifactsTable(s); err != nil {
				return err
			}
			backfillActiveBindings(s)
			backfillNextAttemptNumbers(s)
			return nil
		},
	}
}

func createActiveBindingsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_action_active_bindings (
		generation_action_id TEXT PRIMARY KEY,
		active_attempt_id TEXT NOT NULL DEFAULT '',
		active_primary_artifact_id TEXT NOT NULL DEFAULT '',
		artifact_content_hash TEXT NOT NULL DEFAULT '',
		binding_revision INTEGER NOT NULL DEFAULT 0,
		bound_at TEXT NOT NULL DEFAULT '',
		bound_reason TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_active_bindings_attempt ON desktop_pet_generation_action_active_bindings(active_attempt_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_active_bindings_artifact ON desktop_pet_generation_action_active_bindings(active_primary_artifact_id)")
	return nil
}

func createProviderReceiptsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_provider_receipts (
		id TEXT PRIMARY KEY,
		attempt_id TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		idempotency_key TEXT NOT NULL DEFAULT '',
		provider_request_id TEXT NOT NULL DEFAULT '',
		provider_task_id TEXT NOT NULL DEFAULT '',
		submitted_at TEXT NOT NULL DEFAULT '',
		first_polled_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT '',
		request_hash TEXT NOT NULL DEFAULT '',
		response_hash TEXT NOT NULL DEFAULT '',
		provider_status TEXT NOT NULL DEFAULT '',
		raw_metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_provider_receipts_attempt ON desktop_pet_generation_provider_receipts(attempt_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_provider_receipts_request ON desktop_pet_generation_provider_receipts(provider_request_id)")
	s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_receipts_idempotency ON desktop_pet_generation_provider_receipts(idempotency_key) WHERE idempotency_key != ''")
	return nil
}

func createArtifactPublishJournalTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_artifact_publish_journal (
		id TEXT PRIMARY KEY,
		artifact_id TEXT NOT NULL DEFAULT '',
		attempt_id TEXT NOT NULL DEFAULT '',
		task_id TEXT NOT NULL DEFAULT '',
		task_action_id TEXT NOT NULL DEFAULT '',
		staging_path TEXT NOT NULL DEFAULT '',
		final_path TEXT NOT NULL DEFAULT '',
		content_hash TEXT NOT NULL DEFAULT '',
		storage_key TEXT NOT NULL DEFAULT '',
		journal_status TEXT NOT NULL DEFAULT 'staging',
		error_message TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT ''
	)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_publish_journal_artifact ON desktop_pet_generation_artifact_publish_journal(artifact_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_publish_journal_status ON desktop_pet_generation_artifact_publish_journal(journal_status)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_publish_journal_attempt ON desktop_pet_generation_artifact_publish_journal(attempt_id)")
	return nil
}

func extendReferenceAssetsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_reference_assets", "content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "normalizer_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "subject_box", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "anchor", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "coordinate_space", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "source_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_reference_assets", "storage_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendGenerationTaskActionsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_generation_task_actions", "next_attempt_number", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_submission_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_provider_job_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_success_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_failed_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_input_units", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_output_units", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "estimated_cost", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "actual_cost", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "currency", "TEXT NOT NULL DEFAULT 'CNY'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "pricing_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_task_actions", "planned_call_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func extendGenerationAttemptsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "artifact_role", "TEXT NOT NULL DEFAULT 'primary'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "cancel_reason", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "cancel_requested_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "cancelled_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "request_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "action_spec_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "provider_config_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "prompt_document_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "prompt_content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "negative_prompt_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "actual_cost", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "actual_input_units", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "actual_output_units", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "lease_owner", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "lease_expires_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "heartbeat_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "retry_after_hint", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_action_generation_attempts", "poll_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return nil
}

func extendGenerationArtifactsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_generation_artifacts", "artifact_role", "TEXT NOT NULL DEFAULT 'primary'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_artifacts", "storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_artifacts", "content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_artifacts", "source_reference_asset_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_artifacts", "source_prompt_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_generation_artifacts", "storage_backend", "TEXT NOT NULL DEFAULT 'local'"); err != nil {
		return err
	}
	return nil
}

func backfillActiveBindings(s *Step) {
	s.Execute(`INSERT OR IGNORE INTO desktop_pet_generation_action_active_bindings (
		generation_action_id,
		active_attempt_id,
		active_primary_artifact_id,
		artifact_content_hash,
		binding_revision,
		bound_at,
		bound_reason,
		created_at,
		updated_at
	)
	SELECT
		a.task_action_id,
		a.id,
		COALESCE((
			SELECT g.id FROM desktop_pet_generation_artifacts g
			WHERE g.attempt_id = a.id AND g.is_primary = 1
			AND g.status IN ('saved', 'verified')
			ORDER BY g.created_at DESC LIMIT 1
		), ''),
		COALESCE((
			SELECT g.hash FROM desktop_pet_generation_artifacts g
			WHERE g.attempt_id = a.id AND g.is_primary = 1
			AND g.status IN ('saved', 'verified')
			ORDER BY g.created_at DESC LIMIT 1
		), ''),
		0,
		COALESCE(a.completed_at, a.updated_at),
		'backfill_from_history',
		COALESCE(a.completed_at, a.updated_at),
		COALESCE(a.completed_at, a.updated_at)
	FROM desktop_pet_action_generation_attempts a
	WHERE a.status = 'succeeded'
	AND a.id = (
		SELECT a2.id FROM desktop_pet_action_generation_attempts a2
		WHERE a2.task_action_id = a.task_action_id
		AND a2.status = 'succeeded'
		ORDER BY a2.attempt_number DESC LIMIT 1
	)`)
}

func backfillNextAttemptNumbers(s *Step) {
	s.Execute(`UPDATE desktop_pet_generation_task_actions
	SET next_attempt_number = MAX(current_attempt + 1, (
		SELECT COALESCE(MAX(a.attempt_number), 0) + 1
		FROM desktop_pet_action_generation_attempts a
		WHERE a.task_action_id = desktop_pet_generation_task_actions.id
	))
	WHERE next_attempt_number <= current_attempt`)
}
