package migration

func DesktopPetReleaseDomainMigration() Migration {
	return Migration{
		Version: "202607310021",
		Name:    "add_desktop_pet_release_domain_tables",
		Up: func(s *Step) error {
			if err := createReleaseBuildSnapshotsTable(s); err != nil {
				return err
			}
			if err := createReleaseBuildOperationsTable(s); err != nil {
				return err
			}
			if err := createReleasePublishJournalsTable(s); err != nil {
				return err
			}
			if err := createLegacyPackageMappingsTable(s); err != nil {
				return err
			}
			if err := createLegacyPackageMigrationOperationsTable(s); err != nil {
				return err
			}
			if err := extendPetIdentitiesForRelease(s); err != nil {
				return err
			}
			if err := extendPackageReleasesForReleaseDomain(s); err != nil {
				return err
			}
			if err := extendPackageOperationsForLease(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func createReleaseBuildSnapshotsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_build_snapshots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  quality_gate_id TEXT NOT NULL DEFAULT '',
  quality_gate_hash TEXT NOT NULL DEFAULT '',
  default_action_key TEXT NOT NULL DEFAULT '',
  included_actions_json TEXT NOT NULL DEFAULT '[]',
  package_schema_version INTEGER NOT NULL DEFAULT 2,
  runtime_contract_version TEXT NOT NULL DEFAULT '',
build_config_hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT ''
)
	s.CreateIndex("idx_drbs_pet", "desktop_pet_release_build_snapshots", []string{"pet_id"}, false)
	s.CreateIndex("idx_drbs_task", "desktop_pet_release_build_snapshots", []string{"processing_task_id"}, false)
	s.CreateIndex("idx_drbs_input_hash", "desktop_pet_release_build_snapshots", []string{"build_config_hash"}, false)
	return nil
}

func createReleaseBuildOperationsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_build_operations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  snapshot_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'created',
  stage TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  heartbeat_at TEXT NOT NULL DEFAULT '',
  staging_path_key TEXT NOT NULL DEFAULT '',
  published_path_key TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
result_json TEXT NOT NULL DEFAULT '{}',
started_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
completed_at TEXT NOT NULL DEFAULT '',
  UNIQUE(user_id, idempotency_key)
)`)
	s.CreateIndex("idx_drbo_user", "desktop_pet_release_build_operations", []string{"user_id"}, false)
	s.CreateIndex("idx_drbo_state", "desktop_pet_release_build_operations", []string{"state"}, false)
	s.CreateIndex("idx_drbo_release", "desktop_pet_release_build_operations", []string{"release_id"}, false)
	s.CreateIndex("idx_drbo_lease", "desktop_pet_release_build_operations", []string{"lease_expires_at"}, false)
	return nil
}

func createReleasePublishJournalsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_publish_journals (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT 'snapshot_created',
  content_root_hash TEXT NOT NULL DEFAULT '',
  staging_path TEXT NOT NULL DEFAULT '',
  published_path TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
)
	s.CreateIndex("idx_drpj_operation", "desktop_pet_release_publish_journals", []string{"operation_id"}, false)
	s.CreateIndex("idx_drpj_release", "desktop_pet_release_publish_journals", []string{"release_id"}, false)
	s.CreateIndex("idx_drpj_stage", "desktop_pet_release_publish_journals", []string{"stage"}, false)
	return nil
}

func createLegacyPackageMappingsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_package_mappings (
  id TEXT PRIMARY KEY,
  legacy_package_id TEXT NOT NULL DEFAULT '',
  migrated_pet_id TEXT NOT NULL DEFAULT '',
  migrated_release_id TEXT NOT NULL DEFAULT '',
  migration_status TEXT NOT NULL DEFAULT 'pending',
  source_content_hash TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(legacy_package_id)
)`)
	s.CreateIndex("idx_dlpm_status", "desktop_pet_legacy_package_mappings", []string{"migration_status"}, false)
	return nil
}

func createLegacyPackageMigrationOperationsTable(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_package_migration_operations (
  id TEXT PRIMARY KEY,
  legacy_package_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'pending',
  staging_path TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
completed_at TEXT NOT NULL DEFAULT ''
)`)
	s.CreateIndex("idx_dlpmo_legacy", "desktop_pet_legacy_package_migration_operations", []string{"legacy_package_id"}, false)
	s.CreateIndex("idx_dlpmo_state", "desktop_pet_legacy_package_migration_operations", []string{"state"}, false)
	return nil
}

func extendPetIdentitiesForRelease(s *Step) error {
	if err := s.AddColumn("desktop_pet_identities", "next_release_sequence", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_identities", "default_action_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func extendPackageReleasesForReleaseDomain(s *Step) error {
	if err := s.AddColumn("desktop_pet_package_releases", "active_revision_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "quality_gate_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "quality_gate_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "build_snapshot_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "integrity_status", "TEXT NOT NULL DEFAULT 'unknown'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "compatibility_status", "TEXT NOT NULL DEFAULT 'unknown'"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_releases", "lifecycle", "TEXT NOT NULL DEFAULT 'building'"); err != nil {
		return err
	}
	s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_dprel_pet_seq ON desktop_pet_package_releases(pet_id, release_sequence)")
	return nil
}

func extendPackageOperationsForLease(s *Step) error {
	if err := s.AddColumn("desktop_pet_package_operations", "lease_owner", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_operations", "lease_expires_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_operations", "heartbeat_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.AddColumn("desktop_pet_package_operations", "snapshot_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}
