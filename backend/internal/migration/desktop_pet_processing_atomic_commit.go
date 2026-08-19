package migration

func DesktopPetProcessingAtomicCommitMigration() Migration {
	return Migration{
		Version: "202607311003",
		Name:    "add_desktop_pet_processing_atomic_commit_columns",
		Up: func(s *Step) error {
			if err := extendProcessingTasksForAtomicCommit(s); err != nil {
				return err
			}
			if err := extendProcessingActionAttemptsForAtomicCommit(s); err != nil {
				return err
			}
			if err := extendProcessingRevisionsForAtomicCommit(s); err != nil {
				return err
			}
			if err := extendProcessingSourceManifestsForAtomicCommit(s); err != nil {
				return err
			}
			if err := createProcessingCommitJournalsTable(s); err != nil {
				return err
			}
			if err := createProcessingEventOutboxTable(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func extendProcessingTasksForAtomicCommit(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_tasks", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_tasks", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendProcessingActionAttemptsForAtomicCommit(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "source_generation_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "source_generation_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "source_manifest_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_action_attempts", "source_artifact_content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendProcessingRevisionsForAtomicCommit(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_revisions", "processing_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "source_generation_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "source_generation_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "source_artifact_content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "root_storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_revisions", "committed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendProcessingSourceManifestsForAtomicCommit(s *Step) error {
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "active_artifact_binding_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "artifact_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "reference_asset_content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "generation_plan_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "generation_plan_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_processing_source_manifests", "action_spec_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func createProcessingCommitJournalsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_commit_journals (
  id TEXT PRIMARY KEY,
  commit_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  source_manifest_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  staging_path TEXT NOT NULL DEFAULT '',
  final_path TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  pipeline_result_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT ''
)`)
	s.CreateIndex("idx_dppcj_commit", "desktop_pet_processing_commit_journals", []string{"commit_id"}, false)
	s.CreateIndex("idx_dppcj_attempt", "desktop_pet_processing_commit_journals", []string{"processing_attempt_id"}, false)
	s.CreateIndex("idx_dppcj_status", "desktop_pet_processing_commit_journals", []string{"status"}, false)
	return nil
}

func createProcessingEventOutboxTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_event_outbox (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0
)`)
	s.CreateIndex("idx_dppeo_status", "desktop_pet_processing_event_outbox", []string{"status"}, false)
	s.CreateIndex("idx_dppeo_aggregate", "desktop_pet_processing_event_outbox", []string{"aggregate_id"}, false)
	s.CreateIndex("idx_dppeo_created", "desktop_pet_processing_event_outbox", []string{"created_at"}, false)
	return nil
}
