package migration

func DesktopPetPackageReleaseMigration() Migration {
	return Migration{
		Version:           "202607310007",
		Name:              "add_desktop_pet_package_release_system",
		AcceptedChecksums: []string{"945a99577a235cd6c38ba8c5a55e52b27c221df35931606b31772041c768accc"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_identities (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL DEFAULT '',
    source_character_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL DEFAULT '',
    binding_policy TEXT NOT NULL DEFAULT 'character_locked',
    upstream_pet_id TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
)`)
			s.CreateIndex("idx_dpident_owner", "desktop_pet_identities", []string{"owner_user_id"}, false)
			s.CreateIndex("idx_dpident_character", "desktop_pet_identities", []string{"source_character_id"}, false)
			s.CreateIndex("idx_dpident_slug", "desktop_pet_identities", []string{"slug"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_package_releases (
    id TEXT PRIMARY KEY,
    pet_id TEXT NOT NULL DEFAULT '',
    owner_user_id TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    release_sequence INTEGER NOT NULL DEFAULT 0,
    schema_version INTEGER NOT NULL DEFAULT 2,
    status TEXT NOT NULL DEFAULT 'draft',
    content_root_hash TEXT NOT NULL DEFAULT '',
    manifest_hash TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    archive_storage_key TEXT NOT NULL DEFAULT '',
    total_bytes INTEGER NOT NULL DEFAULT 0,
    file_count INTEGER NOT NULL DEFAULT 0,
    action_count INTEGER NOT NULL DEFAULT 0,
    default_action_key TEXT NOT NULL DEFAULT '',
    min_runtime_version TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'generated',
    source_processing_task TEXT NOT NULL DEFAULT '',
    source_generation_task TEXT NOT NULL DEFAULT '',
    quality_gate_snapshot_id TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    published_at TEXT NOT NULL DEFAULT '',
    legacy_package_id TEXT NOT NULL DEFAULT '',
    legacy_version INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    UNIQUE(pet_id, version)
)`)
			s.CreateIndex("idx_dprel_pet", "desktop_pet_package_releases", []string{"pet_id"}, false)
			s.CreateIndex("idx_dprel_owner", "desktop_pet_package_releases", []string{"owner_user_id"}, false)
			s.CreateIndex("idx_dprel_status", "desktop_pet_package_releases", []string{"status"}, false)
			s.CreateIndex("idx_dprel_content_hash", "desktop_pet_package_releases", []string{"content_root_hash"}, false)
			s.CreateIndex("idx_dprel_legacy", "desktop_pet_package_releases", []string{"legacy_package_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_release_files (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    sha256 TEXT NOT NULL DEFAULT '',
    bytes INTEGER NOT NULL DEFAULT 0,
    media_type TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    frame_id TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    UNIQUE(release_id, path)
)`)
			s.CreateIndex("idx_dprf_release", "desktop_pet_release_files", []string{"release_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_package_operations (
    id TEXT PRIMARY KEY,
    operation_type TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'prepare',
    status TEXT NOT NULL DEFAULT 'pending',
    input_hash TEXT NOT NULL DEFAULT '',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    result_json TEXT NOT NULL DEFAULT '{}',
    started_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, idempotency_key, operation_type)
)`)
			s.CreateIndex("idx_dppkgop_user", "desktop_pet_package_operations", []string{"user_id"}, false)
			s.CreateIndex("idx_dppkgop_status", "desktop_pet_package_operations", []string{"status"}, false)
			s.CreateIndex("idx_dppkgop_release", "desktop_pet_package_operations", []string{"release_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_operations (
    id TEXT PRIMARY KEY,
    operation_type TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    target_release_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'prepare',
    status TEXT NOT NULL DEFAULT 'pending',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    trash_path_key TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    started_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, idempotency_key, operation_type)
)`)
			s.CreateIndex("idx_dpinstop_user", "desktop_pet_installation_operations", []string{"user_id"}, false)
			s.CreateIndex("idx_dpinstop_installation", "desktop_pet_installation_operations", []string{"installation_id"}, false)
			s.CreateIndex("idx_dpinstop_status", "desktop_pet_installation_operations", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_active_bindings (
    user_id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    desired_state TEXT NOT NULL DEFAULT 'disabled',
    runtime_sync_state TEXT NOT NULL DEFAULT 'pending',
    desired_updated_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
)`)
			s.CreateIndex("idx_dpab_installation", "desktop_pet_active_bindings", []string{"installation_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_release_history (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    activated_at TEXT NOT NULL DEFAULT '',
    deactivated_at TEXT NOT NULL DEFAULT '',
    deactivation_reason TEXT NOT NULL DEFAULT '',
    is_current INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
)`)
			s.CreateIndex("idx_dpirh_installation", "desktop_pet_installation_release_history", []string{"installation_id"}, false)
			s.CreateIndex("idx_dpirh_current", "desktop_pet_installation_release_history", []string{"installation_id", "is_current"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_package_validation_reports (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'build',
    verdict TEXT NOT NULL DEFAULT 'pending',
    findings_json TEXT NOT NULL DEFAULT '[]',
    file_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    warning_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
)`)
			s.CreateIndex("idx_dpvr_release", "desktop_pet_package_validation_reports", []string{"release_id"}, false)
			s.CreateIndex("idx_dpvr_operation", "desktop_pet_package_validation_reports", []string{"operation_id"}, false)

			s.AddColumn("desktop_pet_installations", "pet_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "current_release_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "lifecycle_state", "TEXT NOT NULL DEFAULT 'installed'")
			s.AddColumn("desktop_pet_installations", "desired_state", "TEXT NOT NULL DEFAULT 'disabled'")
			s.AddColumn("desktop_pet_installations", "runtime_sync_state", "TEXT NOT NULL DEFAULT 'pending'")
			s.AddColumn("desktop_pet_installations", "state_revision", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_installations", "install_storage_key", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "integrity_root", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "last_error_code", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "last_error_message", "TEXT NOT NULL DEFAULT ''")

			s.CreateIndex("idx_dpinst_pet", "desktop_pet_installations", []string{"pet_id"}, false)
			s.CreateIndex("idx_dpinst_release", "desktop_pet_installations", []string{"current_release_id"}, false)
			s.CreateIndex("idx_dpinst_lifecycle", "desktop_pet_installations", []string{"lifecycle_state"}, false)
			s.CreateIndex("idx_dpinst_desired", "desktop_pet_installations", []string{"desired_state"}, false)

			return nil
		},
	}
}
