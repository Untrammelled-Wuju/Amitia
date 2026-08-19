package migration

func DesktopPetInstallationCoordinatorMigration() Migration {
	return Migration{
		Version: "202607310013",
		Name:    "add_desktop_pet_installation_coordinator_system",
		Up: func(s *Step) error {
			s.AddColumn("desktop_pet_installations", "device_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "preview_artifact_path", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "default_action_release_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "installed_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installations", "integrity_status", "TEXT NOT NULL DEFAULT 'verified'")
			s.AddColumn("desktop_pet_installations", "legacy_package_id", "TEXT NOT NULL DEFAULT ''")
			s.CreateIndex("idx_dpinst_device", "desktop_pet_installations", []string{"device_id"}, false)
			s.CreateIndex("idx_dpinst_user_device_pet", "desktop_pet_installations", []string{"user_id", "device_id", "pet_id"}, false)

			s.AddColumn("desktop_pet_active_bindings", "device_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_active_bindings", "bound_reason", "TEXT NOT NULL DEFAULT 'install'")
			s.AddColumn("desktop_pet_active_bindings", "bound_at", "TEXT NOT NULL DEFAULT ''")
			s.CreateIndex("idx_dpab_user_device", "desktop_pet_active_bindings", []string{"user_id", "device_id"}, false)

			s.AddColumn("desktop_pet_installation_operations", "device_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "request_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "attempt_number", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_installation_operations", "lease_owner", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_installation_operations", "lease_expires_at", "TEXT NOT NULL DEFAULT ''")
			s.CreateIndex("idx_dpinstop_device", "desktop_pet_installation_operations", []string{"device_id"}, false)
			s.CreateIndex("idx_dpinstop_lease", "desktop_pet_installation_operations", []string{"lease_owner", "lease_expires_at"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_states (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    desired_enabled INTEGER NOT NULL DEFAULT 0,
    desired_visible INTEGER NOT NULL DEFAULT 0,
    desired_release_id TEXT NOT NULL DEFAULT '',
    desired_action_key TEXT NOT NULL DEFAULT '',
    position_x REAL,
    position_y REAL,
    scale REAL NOT NULL DEFAULT 1.0,
    opacity REAL NOT NULL DEFAULT 1.0,
    always_on_top INTEGER NOT NULL DEFAULT 1,
    click_through_mode TEXT NOT NULL DEFAULT 'off',
    position_policy TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now')),
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(installation_id)
)`)
			s.CreateIndex("idx_dprds_installation", "desktop_pet_runtime_desired_states", []string{"installation_id"}, false)
			s.CreateIndex("idx_dprds_user", "desktop_pet_runtime_desired_states", []string{"user_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_commit_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    operation_type TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    target_release_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'operation_created',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    trash_path_key TEXT NOT NULL DEFAULT '',
    previous_release_id TEXT NOT NULL DEFAULT '',
    rollback_reason TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpicj_operation", "desktop_pet_installation_commit_journals", []string{"operation_id"}, false)
			s.CreateIndex("idx_dpicj_installation", "desktop_pet_installation_commit_journals", []string{"installation_id"}, false)
			s.CreateIndex("idx_dpicj_state", "desktop_pet_installation_commit_journals", []string{"state"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_switch_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    old_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    old_desired_revision INTEGER NOT NULL DEFAULT 0,
    new_desired_revision INTEGER NOT NULL DEFAULT 0,
    binding_revision INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpisj_operation", "desktop_pet_installation_switch_journals", []string{"operation_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_installation_mappings (
    id TEXT PRIMARY KEY,
    legacy_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    legacy_package_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    migration_status TEXT NOT NULL DEFAULT 'pending',
    source_content_hash TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(legacy_installation_id)
)`)
			s.CreateIndex("idx_dplim_legacy", "desktop_pet_legacy_installation_mappings", []string{"legacy_installation_id"}, false)
			s.CreateIndex("idx_dplim_user", "desktop_pet_legacy_installation_mappings", []string{"user_id"}, false)
			s.CreateIndex("idx_dplim_status", "desktop_pet_legacy_installation_mappings", []string{"migration_status"}, false)

			return nil
		},
	}
}
