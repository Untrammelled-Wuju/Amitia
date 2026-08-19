package migration

func DesktopPetProcessingCommitRecoveryMigration() Migration {
	return Migration{
		Version: "202607310012",
		Name:    "add_desktop_pet_processing_commit_recovery_tables",
		Up: func(s *Step) error {
			if err := createProcessingSourceManifestsTable(s); err != nil {
				return err
			}
			if err := createProcessingPublishJournalsTable(s); err != nil {
				return err
			}
			if err := createProcessingRevisionActiveBindingsTable(s); err != nil {
				return err
			}
			if err := createProcessingWorkspaceLeasesTable(s); err != nil {
				return err
			}
			if err := createProcessingRetryRequestsTable(s); err != nil {
				return err
			}
			if err := extendProcessingRevisionsTable(s); err != nil {
				return err
			}
			if err := extendProcessingActionAttemptsTable(s); err != nil {
				return err
			}
			if err := extendProcessingActionsForRetry(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func createProcessingSourceManifestsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_source_manifests (
  id TEXT PRIMARY KEY,
  schema_version INTEGER NOT NULL DEFAULT 1,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  generation_task_id TEXT NOT NULL DEFAULT '',
  generation_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  generation_mode TEXT NOT NULL DEFAULT '',
  generation_attempt_id TEXT NOT NULL DEFAULT '',
  source_artifact_id TEXT NOT NULL DEFAULT '',
  artifact_role TEXT NOT NULL DEFAULT 'primary',
  artifact_kind TEXT NOT NULL DEFAULT '',
  artifact_content_hash TEXT NOT NULL DEFAULT '',
  artifact_storage_key TEXT NOT NULL DEFAULT '',
  artifact_relative_path TEXT NOT NULL DEFAULT '',
  artifact_width INTEGER NOT NULL DEFAULT 0,
  artifact_height INTEGER NOT NULL DEFAULT 0,
  artifact_mime_type TEXT NOT NULL DEFAULT '',
  candidate_index INTEGER NOT NULL DEFAULT 0,
  reference_asset_id TEXT NOT NULL DEFAULT '',
  prompt_document_id TEXT NOT NULL DEFAULT '',
  prompt_content_hash TEXT NOT NULL DEFAULT '',
  expected_frame_count INTEGER NOT NULL DEFAULT 0,
  sprite_sheet_layout_json TEXT NOT NULL DEFAULT '{}',
  keyframes_json TEXT NOT NULL DEFAULT '[]',
  legacy_frames_json TEXT NOT NULL DEFAULT '[]',
  frames_json TEXT NOT NULL DEFAULT '[]',
  action_spec_snapshot_json TEXT NOT NULL DEFAULT '{}',
  source_config_hash TEXT NOT NULL DEFAULT '',
  manifest_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT ''
)`)
	s.CreateIndex("idx_dppsm_task", "desktop_pet_processing_source_manifests", []string{"processing_task_id"}, false)
	s.CreateIndex("idx_dppsm_gen_action", "desktop_pet_processing_source_manifests", []string{"generation_action_id"}, false)
	s.CreateIndex("uq_dppsm_processing_action", "desktop_pet_processing_source_manifests", []string{"processing_action_id"}, true)
	return nil
}

func createProcessingPublishJournalsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_publish_journals (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  commit_id TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT 'preparing',
  journal_status TEXT NOT NULL DEFAULT 'prepared',
  staging_path TEXT NOT NULL DEFAULT '',
  final_path TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT ''
)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dppj_revision ON desktop_pet_processing_publish_journals(revision_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dppj_action ON desktop_pet_processing_publish_journals(processing_action_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dppj_status ON desktop_pet_processing_publish_journals(journal_status)")
	return nil
}

func createProcessingRevisionActiveBindingsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_revision_active_bindings (
  processing_action_id TEXT PRIMARY KEY,
  active_revision_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  bound_at TEXT NOT NULL DEFAULT '',
  bound_reason TEXT NOT NULL DEFAULT '',
  superseded_revision_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dprab_revision ON desktop_pet_processing_revision_active_bindings(active_revision_id)")
	return nil
}

func createProcessingWorkspaceLeasesTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_workspace_leases (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  commit_id TEXT NOT NULL DEFAULT '',
  workspace_root TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  heartbeat_at TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  cleanup_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dpwl_task ON desktop_pet_processing_workspace_leases(processing_task_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dpwl_attempt ON desktop_pet_processing_workspace_leases(processing_attempt_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dpwl_status ON desktop_pet_processing_workspace_leases(status)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dpwl_lease_expires ON desktop_pet_processing_workspace_leases(lease_expires_at)")
	return nil
}

func createProcessingRetryRequestsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_retry_requests (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL DEFAULT '',
  request_reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'queued',
  allocated_attempt_id TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT ''
)`)
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dprr_task ON desktop_pet_processing_retry_requests(processing_task_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dprr_action ON desktop_pet_processing_retry_requests(processing_action_id)")
	s.Execute("CREATE INDEX IF NOT EXISTS idx_dprr_status ON desktop_pet_processing_retry_requests(status)")
	return nil
}

func extendProcessingRevisionsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_revisions", "content_root_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "activated_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "source_manifest_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "commit_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendProcessingActionAttemptsTable(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "lease_owner", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "lease_expires_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "heartbeat_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "commit_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendProcessingActionsForRetry(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_actions", "pending_retry_request_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_actions", "processing_warnings", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_actions", "warning_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_actions", "action_spec_snapshot", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	return nil
}
