package migration

func DesktopPetReleaseDomainV2Migration() Migration {
	return Migration{
		Version: "202607310022",
		Name:    "add_desktop_pet_release_domain_v2_tables",
		Up: func(s *Step) error {
			if err := createReleaseValidationReportsTableV2(s); err != nil {
				return err
			}
			if err := createReleaseEventOutboxTableV2(s); err != nil {
				return err
			}
			if err := createReleaseBuildRequestInboxTableV2(s); err != nil {
				return err
			}
			if err := createImportPackageSnapshotsTableV2(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func createReleaseValidationReportsTableV2(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_validation_reports (
  id TEXT PRIMARY KEY,
  release_id TEXT NOT NULL DEFAULT '',
  operation_id TEXT NOT NULL DEFAULT '',
  snapshot_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT 'build',
  validator_version TEXT NOT NULL DEFAULT '',
  verdict TEXT NOT NULL DEFAULT 'pending',
  findings_json TEXT NOT NULL DEFAULT '[]',
  file_count INTEGER NOT NULL DEFAULT 0,
  error_count INTEGER NOT NULL DEFAULT 0,
  warning_count INTEGER NOT NULL DEFAULT 0,
  manifest_hash TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
archive_hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
)
	s.CreateIndex("idx_dprvr_release", "desktop_pet_release_validation_reports", []string{"release_id"}, false)
	s.CreateIndex("idx_dprvr_operation", "desktop_pet_release_validation_reports", []string{"operation_id"}, false)
	s.CreateIndex("uq_dprvr_release", "desktop_pet_release_validation_reports", []string{"release_id"}, true)
	return nil
}

func createReleaseEventOutboxTableV2(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'release',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
attempt_count INTEGER NOT NULL DEFAULT 0,
available_at TEXT NOT NULL DEFAULT '',
last_error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
published_at TEXT NOT NULL DEFAULT ''
)`)
	s.CreateIndex("uq_drevo_event_id", "desktop_pet_release_event_outbox", []string{"event_id"}, true)
	s.CreateIndex("uq_drevo_agg_seq_type", "desktop_pet_release_event_outbox", []string{"aggregate_id", "aggregate_sequence", "event_type"}, true)
	s.CreateIndex("idx_drevo_status", "desktop_pet_release_event_outbox", []string{"status"}, false)
	return nil
}

func createReleaseBuildRequestInboxTableV2(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_build_request_inbox (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
operation_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
processed_at TEXT NOT NULL DEFAULT '',
last_error TEXT NOT NULL DEFAULT ''
)`)
	s.CreateIndex("uq_drbrbi_request_id", "desktop_pet_release_build_request_inbox", []string{"request_id"}, true)
	s.CreateIndex("uq_drbrbi_idempotent", "desktop_pet_release_build_request_inbox", []string{"user_id", "idempotency_key"}, true)
	s.CreateIndex("idx_drbrbi_status", "desktop_pet_release_build_request_inbox", []string{"status"}, false)
	return nil
}

func createImportPackageSnapshotsTableV2(s *Step) error {
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_import_package_snapshots (
  id TEXT PRIMARY KEY,
  import_staging_id TEXT NOT NULL DEFAULT '',
  source_package_hash TEXT NOT NULL DEFAULT '',
  source_manifest_hash TEXT NOT NULL DEFAULT '',
  source_schema_version INTEGER NOT NULL DEFAULT 0,
  normalization_warnings TEXT NOT NULL DEFAULT '',
  selected_actions_json TEXT NOT NULL DEFAULT '[]',
  binding_decision TEXT NOT NULL DEFAULT '',
  license_decision TEXT NOT NULL DEFAULT '',
  runtime_compatibility TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
release_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
)
	s.CreateIndex("uq_dips_staging", "desktop_pet_import_package_snapshots", []string{"import_staging_id"}, true)
	s.CreateIndex("idx_dips_release", "desktop_pet_import_package_snapshots", []string{"release_id"}, false)
	return nil
}
