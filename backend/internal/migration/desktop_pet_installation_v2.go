// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetInstallationV2Migration() Migration {
	return Migration{
		Version: "202608020005",
		Name:    "add_desktop_pet_installation_v2_bindings_desired_projection_outbox",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_device_active_installation_bindings (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    bound_reason TEXT NOT NULL DEFAULT 'install_bound',
    bound_at TEXT NOT NULL DEFAULT '',
    bound_by TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(user_id, device_id)
)`)
			if err := s.CreateIndex("idx_dpdainst_installation", "desktop_pet_device_active_installation_bindings", []string{"installation_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpdainst_pet", "desktop_pet_device_active_installation_bindings", []string{"pet_id"}, false); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_device_installation_binding_history (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    previous_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dpbih_user_device", "desktop_pet_device_installation_binding_history", []string{"user_id", "device_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpbih_operation", "desktop_pet_device_installation_binding_history", []string{"operation_id"}, false); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_device_desired_revision_counters (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    current_revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(user_id, device_id)
)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_runtime_projections (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    applied_desired_revision INTEGER NOT NULL DEFAULT 0,
    applied_settings_revision INTEGER NOT NULL DEFAULT 0,
    actual_release_id TEXT NOT NULL DEFAULT '',
    actual_visible INTEGER NOT NULL DEFAULT 0,
    actual_action_key TEXT NOT NULL DEFAULT '',
    actual_health TEXT NOT NULL DEFAULT 'unknown',
    runtime_sync_state TEXT NOT NULL DEFAULT 'pending',
    last_applied_at TEXT NOT NULL DEFAULT '',
    last_heartbeat_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			if err := s.CreateIndex("uq_drirp_user_device", "desktop_pet_installation_runtime_projections", []string{"user_id", "device_id"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_drirp_installation", "desktop_pet_installation_runtime_projections", []string{"installation_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_drirp_runtime", "desktop_pet_installation_runtime_projections", []string{"runtime_id"}, false); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_state_outbox (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL DEFAULT 'desired_state_changed',
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    desired_revision INTEGER NOT NULL DEFAULT 0,
    desired_hash TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    available_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    published_at TEXT NOT NULL DEFAULT ''
)`)
			if err := s.CreateIndex("idx_drdso_user_device_status", "desktop_pet_runtime_desired_state_outbox", []string{"user_id", "device_id", "status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_drdso_available", "desktop_pet_runtime_desired_state_outbox", []string{"status", "available_at"}, false); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_trash_entries (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    storage_key TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    retain_until TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now')),
    purged_at TEXT NOT NULL DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dptein_installation", "desktop_pet_installation_trash_entries", []string{"installation_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dptein_retain", "desktop_pet_installation_trash_entries", []string{"retain_until"}, false); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_commit_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    source_release_id TEXT NOT NULL DEFAULT '',
    target_release_id TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'operation_created',
    status TEXT NOT NULL DEFAULT 'active',
    execution_id TEXT NOT NULL DEFAULT '',
    expected_old_status TEXT NOT NULL DEFAULT '',
    staging_path_key TEXT NOT NULL DEFAULT '',
    rollback_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    trash_path_key TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			if err := s.CreateIndex("idx_dpcj_operation", "desktop_pet_installation_commit_journals", []string{"operation_id"}, true); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installation_switch_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    old_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    old_binding_revision INTEGER NOT NULL DEFAULT 0,
    new_binding_revision INTEGER NOT NULL DEFAULT 0,
    old_desired_revision INTEGER NOT NULL DEFAULT 0,
    new_desired_revision INTEGER NOT NULL DEFAULT 0,
    old_release_id TEXT NOT NULL DEFAULT '',
    new_release_id TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'created',
    status TEXT NOT NULL DEFAULT 'active',
    execution_id TEXT NOT NULL DEFAULT '',
    expected_old_stage TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			if err := s.CreateIndex("idx_dpsj_operation", "desktop_pet_installation_switch_journals", []string{"operation_id"}, true); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_installation_mappings (
    id TEXT PRIMARY KEY,
    legacy_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    migration_status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
)`)
			if err := s.CreateIndex("idx_dplim_legacy", "desktop_pet_legacy_installation_mappings", []string{"legacy_installation_id"}, true); err != nil {
				return err
			}

			if err := s.CreateIndex("uq_dpinst_user_device_pet", "desktop_pet_installations", []string{"user_id", "device_id", "pet_id"}, true); err != nil {
				return err
			}

			return nil
		},
	}
}
