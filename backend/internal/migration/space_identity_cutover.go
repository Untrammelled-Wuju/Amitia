package migration

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/spaceidentity"
)

type identityColumnRename struct {
	Table string
	Old   string
	New   string
}

type spaceOwnershipColumn struct {
	Table  string
	Column string
}

var legacyAccountTables = map[string]struct{}{
	"auth_users":           {},
	"auth_sessions":        {},
	"auth_refresh_tokens":  {},
	"auth_login_guards":    {},
	"auth_recovery_codes":  {},
	"auth_recovery_grants": {},
}

var legacyAccountReferencePattern = regexp.MustCompile(`(?i)\breferences\s+(?:auth_users|auth_sessions|auth_refresh_tokens|auth_login_guards|auth_recovery_codes|auth_recovery_grants|"auth_users"|"auth_sessions"|"auth_refresh_tokens"|"auth_login_guards"|"auth_recovery_codes"|"auth_recovery_grants"|` + "`" + `auth_users` + "`" + `|` + "`" + `auth_sessions` + "`" + `|` + "`" + `auth_refresh_tokens` + "`" + `|` + "`" + `auth_login_guards` + "`" + `|` + "`" + `auth_recovery_codes` + "`" + `|` + "`" + `auth_recovery_grants` + "`" + `|\[auth_users\]|\[auth_sessions\]|\[auth_refresh_tokens\]|\[auth_login_guards\]|\[auth_recovery_codes\]|\[auth_recovery_grants\])\s*\([^)]*\)(?:(?:\s+ON\s+(?:DELETE|UPDATE)\s+(?:CASCADE|RESTRICT|SET\s+NULL|SET\s+DEFAULT|NO\s+ACTION))|(?:\s+MATCH\s+\w+)|(?:\s+NOT\s+DEFERRABLE)|(?:\s+DEFERRABLE(?:\s+INITIALLY\s+(?:DEFERRED|IMMEDIATE))?))*`)

var spaceIdentityColumnRenames = []identityColumnRename{
	{Table: "artifacts", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "characters", Old: "user_id", New: "space_id"},
	{Table: "conversations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_action_active_revisions", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_action_revisions", Old: "created_by_user_id", New: "created_by_space_id"},
	{Table: "desktop_pet_action_revisions", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_action_streams", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_active_action_revision_bindings", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_active_bindings", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_bindings", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_contexts", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_cooldowns", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_decisions", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_inbox", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_behavior_mesh_affinities", Old: "cloud_user_id", New: "cloud_space_id"},
	{Table: "desktop_pet_behavior_mesh_outbox", Old: "cloud_user_id", New: "cloud_space_id"},
	{Table: "desktop_pet_candidate_acceptance_operations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_device_active_installation_bindings", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_device_desired_revision_counters", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_device_installation_binding_history", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_devices", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_edit_audit_logs", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_edit_candidates", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_edit_draft_snapshots", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_edit_idempotency", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_edit_sessions", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_editing_event_outbox", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_frame_assets", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_generation_tasks", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_identities", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "desktop_pet_import_package_snapshots", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_import_stagings", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "desktop_pet_installation_commit_journals", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_installation_operations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_installation_runtime_projections", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_installation_switch_journals", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_installations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_legacy_installation_mappings", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_legacy_package_mappings", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "desktop_pet_legacy_package_migration_operations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_local_sessions", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_owner_mappings", Old: "cloud_user_id", New: "cloud_space_id"},
	{Table: "desktop_pet_package_operations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_package_releases", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "desktop_pet_packages", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_processing_source_manifests", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_processing_tasks", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_quality_evaluations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_quality_gate_snapshots", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_quality_input_snapshots", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_reference_assets", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_regeneration_jobs", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_release_build_operations", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_release_build_request_inbox", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_release_build_snapshots", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_revision_bridge_journals", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_actual_states_v2", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_bootstrap_tickets", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_clients", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_command_dedup", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_commands", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_commands_v2", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_desired_state_outbox", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_desired_states", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_device_command_sequences", Old: "user_id", New: "space_id"},
	{Table: "desktop_pet_runtime_sessions", Old: "user_id", New: "space_id"},
	{Table: "emotion_states", Old: "user_id", New: "space_id"},
	{Table: "episodic_memories", Old: "user_id", New: "space_id"},
	{Table: "extension_agent_skill_activations", Old: "user_id", New: "space_id"},
	{Table: "extension_agent_skill_metadata", Old: "user_id", New: "space_id"},
	{Table: "extension_package_exports", Old: "user_id", New: "space_id"},
	{Table: "extension_package_import_sessions", Old: "user_id", New: "space_id"},
	{Table: "extension_package_installations", Old: "user_id", New: "space_id"},
	{Table: "extension_runs", Old: "user_id", New: "space_id"},
	{Table: "extension_workshop_sessions", Old: "user_id", New: "space_id"},
	{Table: "extension_workshop_test_runs", Old: "user_id", New: "space_id"},
	{Table: "extensions", Old: "owner_user_id", New: "owner_space_id"},
	{Table: "interaction_records", Old: "user_id", New: "space_id"},
	{Table: "kernel_device_runtime_sessions", Old: "user_id", New: "space_id"},
	{Table: "memories", Old: "user_id", New: "space_id"},
	{Table: "memory_candidates", Old: "user_id", New: "space_id"},
	{Table: "output_leases", Old: "user_id", New: "space_id"},
	{Table: "reminders", Old: "user_id", New: "space_id"},
	{Table: "retrieval_logs", Old: "user_id", New: "space_id"},
	{Table: "sync_changes", Old: "user_id", New: "space_id"},
	{Table: "sync_cursors", Old: "user_id", New: "space_id"},
	{Table: "sync_mutation_claims", Old: "user_id", New: "space_id"},
	{Table: "temporal_anchors", Old: "user_id", New: "space_id"},
	{Table: "temporal_cadence_samples", Old: "user_id", New: "space_id"},
	{Table: "temporal_effect_ledger", Old: "user_id", New: "space_id"},
	{Table: "temporal_events", Old: "user_id", New: "space_id"},
	{Table: "temporal_global_presence_states", Old: "user_id", New: "space_id"},
	{Table: "temporal_interaction_receipts", Old: "user_id", New: "space_id"},
	{Table: "temporal_relationship_presence_states", Old: "user_id", New: "space_id"},
	{Table: "temporal_reunion_episodes", Old: "user_id", New: "space_id"},
	{Table: "trigger_histories", Old: "user_id", New: "space_id"},
	{Table: "tts_cloned_voices", Old: "user_id", New: "space_id"},
	{Table: "user_profiles", Old: "user_id", New: "space_id"},
	{Table: "world_book", Old: "user_id", New: "space_id"},
}

var canonicalSpaceOwnershipColumns = []spaceOwnershipColumn{
	{Table: "artifacts", Column: "owner_space_id"},
	{Table: "characters", Column: "space_id"},
	{Table: "conversations", Column: "space_id"},
	{Table: "desktop_pet_action_active_revisions", Column: "space_id"},
	{Table: "desktop_pet_action_revisions", Column: "created_by_space_id"},
	{Table: "desktop_pet_action_revisions", Column: "space_id"},
	{Table: "desktop_pet_action_streams", Column: "space_id"},
	{Table: "desktop_pet_active_action_revision_bindings", Column: "space_id"},
	{Table: "desktop_pet_active_bindings", Column: "space_id"},
	{Table: "desktop_pet_behavior_bindings", Column: "space_id"},
	{Table: "desktop_pet_behavior_contexts", Column: "space_id"},
	{Table: "desktop_pet_behavior_cooldowns", Column: "space_id"},
	{Table: "desktop_pet_behavior_decisions", Column: "space_id"},
	{Table: "desktop_pet_behavior_inbox", Column: "space_id"},
	{Table: "desktop_pet_candidate_acceptance_operations", Column: "space_id"},
	{Table: "desktop_pet_device_active_installation_bindings", Column: "space_id"},
	{Table: "desktop_pet_device_desired_revision_counters", Column: "space_id"},
	{Table: "desktop_pet_device_installation_binding_history", Column: "space_id"},
	{Table: "desktop_pet_devices", Column: "space_id"},
	{Table: "desktop_pet_edit_audit_logs", Column: "space_id"},
	{Table: "desktop_pet_edit_candidates", Column: "space_id"},
	{Table: "desktop_pet_edit_draft_snapshots", Column: "space_id"},
	{Table: "desktop_pet_edit_idempotency", Column: "space_id"},
	{Table: "desktop_pet_edit_sessions", Column: "space_id"},
	{Table: "desktop_pet_editing_event_outbox", Column: "space_id"},
	{Table: "desktop_pet_frame_assets", Column: "space_id"},
	{Table: "desktop_pet_generation_tasks", Column: "space_id"},
	{Table: "desktop_pet_identities", Column: "owner_space_id"},
	{Table: "desktop_pet_import_package_snapshots", Column: "space_id"},
	{Table: "desktop_pet_import_stagings", Column: "owner_space_id"},
	{Table: "desktop_pet_installation_commit_journals", Column: "space_id"},
	{Table: "desktop_pet_installation_operations", Column: "space_id"},
	{Table: "desktop_pet_installation_runtime_projections", Column: "space_id"},
	{Table: "desktop_pet_installation_switch_journals", Column: "space_id"},
	{Table: "desktop_pet_installations", Column: "space_id"},
	{Table: "desktop_pet_legacy_installation_mappings", Column: "space_id"},
	{Table: "desktop_pet_legacy_package_mappings", Column: "owner_space_id"},
	{Table: "desktop_pet_legacy_package_migration_operations", Column: "space_id"},
	{Table: "desktop_pet_local_sessions", Column: "space_id"},
	{Table: "desktop_pet_package_operations", Column: "space_id"},
	{Table: "desktop_pet_package_releases", Column: "owner_space_id"},
	{Table: "desktop_pet_packages", Column: "space_id"},
	{Table: "desktop_pet_processing_source_manifests", Column: "space_id"},
	{Table: "desktop_pet_processing_tasks", Column: "space_id"},
	{Table: "desktop_pet_quality_evaluations", Column: "space_id"},
	{Table: "desktop_pet_quality_gate_snapshots", Column: "space_id"},
	{Table: "desktop_pet_quality_input_snapshots", Column: "space_id"},
	{Table: "desktop_pet_reference_assets", Column: "space_id"},
	{Table: "desktop_pet_regeneration_jobs", Column: "space_id"},
	{Table: "desktop_pet_release_build_operations", Column: "space_id"},
	{Table: "desktop_pet_release_build_request_inbox", Column: "space_id"},
	{Table: "desktop_pet_release_build_snapshots", Column: "space_id"},
	{Table: "desktop_pet_revision_bridge_journals", Column: "space_id"},
	{Table: "desktop_pet_runtime_actual_states_v2", Column: "space_id"},
	{Table: "desktop_pet_runtime_bootstrap_tickets", Column: "space_id"},
	{Table: "desktop_pet_runtime_clients", Column: "space_id"},
	{Table: "desktop_pet_runtime_command_dedup", Column: "space_id"},
	{Table: "desktop_pet_runtime_commands", Column: "space_id"},
	{Table: "desktop_pet_runtime_commands_v2", Column: "space_id"},
	{Table: "desktop_pet_runtime_desired_state_outbox", Column: "space_id"},
	{Table: "desktop_pet_runtime_desired_states", Column: "space_id"},
	{Table: "desktop_pet_runtime_device_command_sequences", Column: "space_id"},
	{Table: "desktop_pet_runtime_sessions", Column: "space_id"},
	{Table: "emotion_states", Column: "space_id"},
	{Table: "episodic_memories", Column: "space_id"},
	{Table: "extension_agent_skill_activations", Column: "space_id"},
	{Table: "extension_agent_skill_metadata", Column: "space_id"},
	{Table: "extension_package_exports", Column: "space_id"},
	{Table: "extension_package_import_sessions", Column: "space_id"},
	{Table: "extension_package_installations", Column: "space_id"},
	{Table: "extension_runs", Column: "space_id"},
	{Table: "extension_workshop_sessions", Column: "space_id"},
	{Table: "extension_workshop_test_runs", Column: "space_id"},
	{Table: "extensions", Column: "owner_space_id"},
	{Table: "interaction_records", Column: "space_id"},
	{Table: "kernel_device_runtime_sessions", Column: "space_id"},
	{Table: "memories", Column: "space_id"},
	{Table: "memory_candidates", Column: "space_id"},
	{Table: "output_leases", Column: "space_id"},
	{Table: "reminders", Column: "space_id"},
	{Table: "retrieval_logs", Column: "space_id"},
	{Table: "sync_changes", Column: "space_id"},
	{Table: "sync_cursors", Column: "space_id"},
	{Table: "sync_mutation_claims", Column: "space_id"},
	{Table: "temporal_anchors", Column: "space_id"},
	{Table: "temporal_cadence_samples", Column: "space_id"},
	{Table: "temporal_effect_ledger", Column: "space_id"},
	{Table: "temporal_events", Column: "space_id"},
	{Table: "temporal_global_presence_states", Column: "space_id"},
	{Table: "temporal_interaction_receipts", Column: "space_id"},
	{Table: "temporal_relationship_presence_states", Column: "space_id"},
	{Table: "temporal_reunion_episodes", Column: "space_id"},
	{Table: "trigger_histories", Column: "space_id"},
	{Table: "tts_cloned_voices", Column: "space_id"},
	{Table: "user_profiles", Column: "space_id"},
	{Table: "world_book", Column: "space_id"},
}

// SpaceIdentityCutoverMigration is the terminal migration from the legacy
// account/user ownership model to the accountless Space + Device model.
// Historical migrations remain untouched so released databases can still
// replay their original schema history; this cutover removes the legacy
// identity columns and account tables after that history has been applied.
func SpaceIdentityCutoverMigration() Migration {
	checksum := func(s *Step) error {
		s.Execute("SELECT 1 /* space_identity_cutover_v1 */")
		return nil
	}
	return Migration{
		Version:    "20260914004",
		Name:       "cutover_account_identity_to_space_device_model",
		ChecksumUp: checksum,
		Up: func(s *Step) error {
			if s == nil || s.DB() == nil {
				return fmt.Errorf("space identity cutover: database is required")
			}
			// Keep the checksum deterministic even though the migration uses
			// direct transactional schema inspection for compatibility.
			if err := checksum(s); err != nil {
				return err
			}

			if err := migrateLegacyAccountProfile(s); err != nil {
				return err
			}
			if err := detachLegacyAccountForeignKeys(s); err != nil {
				return err
			}

			for _, item := range spaceIdentityColumnRenames {
				if err := renameIdentityColumn(s, item.Table, item.Old, item.New); err != nil {
					return err
				}
			}
			if err := migrateLegacyIdentityEnumValues(s); err != nil {
				return err
			}
			if err := rebuildSecurityAuditEventsForSpace(s); err != nil {
				return err
			}

			canonical := strings.TrimSpace(spaceidentity.DefaultSpaceID())
			if canonical != "" {
				for _, item := range canonicalSpaceOwnershipColumns {
					if err := bindColumnToCanonicalSpace(s, item.Table, item.Column, canonical); err != nil {
						return err
					}
				}
				if err := bindColumnToCanonicalSpace(s, "security_audit_events", "space_id", canonical); err != nil {
					return err
				}
			}

			// Account authentication is intentionally not preserved as a
			// compatibility path. External-provider OAuth sessions (for MCP,
			// chat channels, etc.) live in their own service-connection tables
			// and are not part of this list.
			for _, table := range []string{
				"auth_recovery_grants",
				"auth_recovery_codes",
				"auth_login_guards",
				"auth_refresh_tokens",
				"auth_sessions",
				"auth_users",
			} {
				if err := dropLegacyAccountTable(s, table); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func migrateLegacyIdentityEnumValues(s *Step) error {
	updates := []struct {
		table  string
		column string
	}{
		{table: "temporal_profiles", column: "owner_type"},
		{table: "temporal_anchors", column: "scope_type"},
	}
	for _, item := range updates {
		exists, err := s.TableExists(item.table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		columnExists, err := s.ColumnExists(item.table, item.column)
		if err != nil {
			return err
		}
		if !columnExists {
			continue
		}
		if err := s.DB().Exec(
			"UPDATE " + item.table + " SET " + item.column + " = 'space' WHERE LOWER(TRIM(COALESCE(" + item.column + ", ''))) = 'user'",
		).Error; err != nil {
			return fmt.Errorf("space identity cutover: migrate %s.%s user to space: %w", item.table, item.column, err)
		}
	}
	return nil
}

func renameIdentityColumn(s *Step, table, oldColumn, newColumn string) error {
	exists, err := s.TableExists(table)
	if err != nil || !exists {
		return err
	}
	oldExists, err := s.ColumnExists(table, oldColumn)
	if err != nil {
		return err
	}
	newExists, err := s.ColumnExists(table, newColumn)
	if err != nil {
		return err
	}
	if !oldExists {
		return nil
	}
	if !newExists {
		if err := s.DB().Exec("ALTER TABLE " + table + " RENAME COLUMN " + oldColumn + " TO " + newColumn).Error; err != nil {
			return fmt.Errorf("space identity cutover: rename %s.%s to %s: %w", table, oldColumn, newColumn, err)
		}
		return nil
	}

	// A pre-release build may already have added the Space column while still
	// writing the legacy column. Merge first, then remove the legacy column so
	// there is only one ownership truth source.
	if err := s.DB().Exec(
		"UPDATE " + table + " SET " + newColumn + " = " + oldColumn +
			" WHERE (TRIM(COALESCE(CAST(" + newColumn + " AS TEXT), '')) = '' OR " + newColumn + " = 'default')" +
			" AND TRIM(COALESCE(CAST(" + oldColumn + " AS TEXT), '')) <> ''",
	).Error; err != nil {
		return fmt.Errorf("space identity cutover: merge %s.%s: %w", table, oldColumn, err)
	}
	if err := s.DB().Exec("ALTER TABLE " + table + " DROP COLUMN " + oldColumn).Error; err != nil {
		return fmt.Errorf("space identity cutover: drop legacy column %s.%s: %w", table, oldColumn, err)
	}
	return nil
}

func bindColumnToCanonicalSpace(s *Step, table, column, canonical string) error {
	exists, err := s.TableExists(table)
	if err != nil || !exists {
		return err
	}
	columnExists, err := s.ColumnExists(table, column)
	if err != nil || !columnExists {
		return err
	}
	if err := s.DB().Exec("UPDATE "+table+" SET "+column+" = ? WHERE "+column+" IS NULL OR TRIM(CAST("+column+" AS TEXT)) <> ?", canonical, canonical).Error; err != nil {
		return fmt.Errorf("space identity cutover: canonicalize %s.%s: %w", table, column, err)
	}
	return nil
}

func detachLegacyAccountForeignKeys(s *Step) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("space identity cutover: database is required")
	}
	type schemaRow struct {
		Name string `gorm:"column:name"`
		SQL  string `gorm:"column:sql"`
	}
	var tables []schemaRow
	if err := s.DB().Raw(`SELECT name, sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&tables).Error; err != nil {
		return fmt.Errorf("space identity cutover: inspect sqlite schema: %w", err)
	}

	updates := make([]schemaRow, 0)
	for _, table := range tables {
		hasLegacyFK, err := tableReferencesLegacyAccount(s, table.Name)
		if err != nil {
			return err
		}
		if !hasLegacyFK {
			continue
		}
		cleaned, err := stripLegacyAccountReferences(table.SQL)
		if err != nil {
			return fmt.Errorf("space identity cutover: detach legacy foreign keys from %s: %w", table.Name, err)
		}
		if strings.TrimSpace(cleaned) == strings.TrimSpace(table.SQL) {
			return fmt.Errorf("space identity cutover: legacy account foreign key remains in %s but schema could not be rewritten", table.Name)
		}
		updates = append(updates, schemaRow{Name: table.Name, SQL: cleaned})
	}
	if len(updates) == 0 {
		return nil
	}

	var schemaVersion int
	if err := s.DB().Raw("PRAGMA schema_version").Scan(&schemaVersion).Error; err != nil {
		return fmt.Errorf("space identity cutover: read schema version: %w", err)
	}
	if err := s.DB().Exec("PRAGMA writable_schema=ON").Error; err != nil {
		return fmt.Errorf("space identity cutover: enable sqlite schema rewrite: %w", err)
	}
	writable := true
	defer func() {
		if writable {
			_ = s.DB().Exec("PRAGMA writable_schema=OFF").Error
		}
	}()
	for _, update := range updates {
		if err := s.DB().Exec("UPDATE sqlite_master SET sql = ? WHERE type = 'table' AND name = ?", update.SQL, update.Name).Error; err != nil {
			return fmt.Errorf("space identity cutover: rewrite schema for %s: %w", update.Name, err)
		}
	}
	if err := s.DB().Exec("PRAGMA writable_schema=OFF").Error; err != nil {
		return fmt.Errorf("space identity cutover: disable sqlite schema rewrite: %w", err)
	}
	writable = false
	if err := s.DB().Exec(fmt.Sprintf("PRAGMA schema_version=%d", schemaVersion+1)).Error; err != nil {
		return fmt.Errorf("space identity cutover: refresh sqlite schema: %w", err)
	}
	for _, update := range updates {
		stillReferences, err := tableReferencesLegacyAccount(s, update.Name)
		if err != nil {
			return err
		}
		if stillReferences {
			return fmt.Errorf("space identity cutover: legacy account foreign key still present in %s", update.Name)
		}
	}
	return nil
}

func tableReferencesLegacyAccount(s *Step, table string) (bool, error) {
	type foreignKeyRow struct {
		ReferencedTable string `gorm:"column:table"`
	}
	var refs []foreignKeyRow
	quoted := strings.ReplaceAll(table, `"`, `""`)
	if err := s.DB().Raw(`PRAGMA foreign_key_list("` + quoted + `")`).Scan(&refs).Error; err != nil {
		return false, fmt.Errorf("space identity cutover: inspect foreign keys for %s: %w", table, err)
	}
	for _, ref := range refs {
		if _, ok := legacyAccountTables[strings.ToLower(strings.TrimSpace(ref.ReferencedTable))]; ok {
			return true, nil
		}
	}
	return false, nil
}

func stripLegacyAccountReferences(createSQL string) (string, error) {
	open := strings.Index(createSQL, "(")
	if open < 0 {
		return "", fmt.Errorf("invalid CREATE TABLE SQL")
	}
	close := matchingCreateTableClose(createSQL, open)
	if close <= open {
		return "", fmt.Errorf("invalid CREATE TABLE body")
	}
	body := createSQL[open+1 : close]
	parts := splitSQLTopLevel(body)
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if legacyAccountReferencePattern.MatchString(trimmed) {
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "foreign key") || (strings.HasPrefix(lower, "constraint ") && strings.Contains(lower, " foreign key")) {
				continue
			}
			trimmed = strings.TrimSpace(legacyAccountReferencePattern.ReplaceAllString(trimmed, ""))
			if trimmed == "" {
				return "", fmt.Errorf("column definition became empty after removing legacy account reference")
			}
		}
		cleaned = append(cleaned, trimmed)
	}
	result := createSQL[:open+1] + strings.Join(cleaned, ", ") + createSQL[close:]
	if legacyAccountReferencePattern.MatchString(result) {
		return "", fmt.Errorf("unsupported legacy account foreign-key syntax")
	}
	return result, nil
}

func matchingCreateTableClose(sql string, open int) int {
	depth := 0
	quote := byte(0)
	bracket := false
	for i := open; i < len(sql); i++ {
		ch := sql[i]
		if bracket {
			if ch == ']' {
				bracket = false
			}
			continue
		}
		if quote != 0 {
			if ch == quote {
				if i+1 < len(sql) && sql[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '[':
			bracket = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitSQLTopLevel(body string) []string {
	parts := make([]string, 0)
	start := 0
	depth := 0
	quote := byte(0)
	bracket := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if bracket {
			if ch == ']' {
				bracket = false
			}
			continue
		}
		if quote != 0 {
			if ch == quote {
				if i+1 < len(body) && body[i+1] == quote {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '[':
			bracket = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, body[start:])
	return parts
}

func dropLegacyAccountTable(s *Step, table string) error {
	exists, err := s.TableExists(table)
	if err != nil || !exists {
		return err
	}
	if err := s.DB().Exec("DROP TABLE " + table).Error; err != nil {
		return fmt.Errorf("space identity cutover: drop %s: %w", table, err)
	}
	return nil
}

func migrateLegacyAccountProfile(s *Step) error {
	exists, err := s.TableExists("auth_users")
	if err != nil || !exists {
		return err
	}
	store := spaceidentity.DefaultStore()
	if store == nil {
		return nil
	}

	current, err := store.ReadProfile()
	if err != nil {
		return fmt.Errorf("space identity cutover: read profile: %w", err)
	}
	if strings.TrimSpace(current.DisplayName) != "" || strings.TrimSpace(current.UserLabel) != "" || strings.TrimSpace(current.Bio) != "" {
		return nil
	}

	cols := map[string]bool{}
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.DB().Raw("PRAGMA table_info(auth_users)").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		cols[row.Name] = true
	}
	expr := func(name string) string {
		if cols[name] {
			return "COALESCE(" + name + ", '')"
		}
		return "''"
	}
	order := "rowid"
	if cols["is_active"] && cols["id"] {
		order = "CASE WHEN is_active = 1 THEN 0 ELSE 1 END, id"
	} else if cols["id"] {
		order = "id"
	}
	query := "SELECT " + expr("nickname") + " AS nickname, " + expr("user_label") + " AS user_label, " + expr("bio") + " AS bio, " + expr("username") + " AS username FROM auth_users ORDER BY " + order + " LIMIT 1"
	var legacy struct {
		Nickname  string `gorm:"column:nickname"`
		UserLabel string `gorm:"column:user_label"`
		Bio       string `gorm:"column:bio"`
		Username  string `gorm:"column:username"`
	}
	if err := s.DB().Raw(query).Scan(&legacy).Error; err != nil {
		return fmt.Errorf("space identity cutover: read legacy profile: %w", err)
	}
	displayName := strings.TrimSpace(legacy.Nickname)
	if displayName == "" {
		displayName = strings.TrimSpace(legacy.Username)
	}
	if displayName == "" && strings.TrimSpace(legacy.UserLabel) == "" && strings.TrimSpace(legacy.Bio) == "" {
		return nil
	}
	current.DisplayName = displayName
	current.UserLabel = strings.TrimSpace(legacy.UserLabel)
	current.Bio = strings.TrimSpace(legacy.Bio)
	if err := store.WriteProfile(current); err != nil {
		return fmt.Errorf("space identity cutover: write profile: %w", err)
	}
	return nil
}

func rebuildSecurityAuditEventsForSpace(s *Step) error {
	exists, err := s.TableExists("security_audit_events")
	if err != nil || !exists {
		return err
	}

	cols := map[string]bool{}
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.DB().Raw("PRAGMA table_info(security_audit_events)").Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		cols[row.Name] = true
	}

	// Already on the canonical schema: no destructive rebuild is required.
	canonicalColumns := []string{"event_id", "event_type", "severity", "outcome", "space_id", "device_id", "runtime_id", "session_id", "principal_type", "auth_method", "ip_address", "user_agent", "reason_code", "details_json", "occurred_at"}
	exact := len(cols) == len(canonicalColumns)
	if exact {
		for _, name := range canonicalColumns {
			if !cols[name] {
				exact = false
				break
			}
		}
	}
	if exact {
		return nil
	}

	pick := func(name, fallback string) string {
		if cols[name] {
			return "COALESCE(" + name + ", " + fallback + ")"
		}
		return fallback
	}
	eventID := pick("event_id", "''")
	if cols["id"] {
		eventID = "COALESCE(NULLIF(" + eventID + ", ''), 'legacy-audit-' || CAST(id AS TEXT))"
	}
	principal := pick("principal_type", "''")
	if !cols["principal_type"] && cols["actor_type"] {
		principal = pick("actor_type", "''")
	}
	occurred := pick("occurred_at", "''")
	if !cols["occurred_at"] && cols["created_at"] {
		occurred = pick("created_at", "''")
	}
	details := pick("details_json", "''")
	if !cols["details_json"] && cols["detail"] {
		details = pick("detail", "''")
	}

	if err := s.DB().Exec("DROP TABLE IF EXISTS security_audit_events_space_cutover").Error; err != nil {
		return err
	}
	if err := s.DB().Exec(`CREATE TABLE security_audit_events_space_cutover (
        event_id TEXT PRIMARY KEY,
        event_type TEXT NOT NULL,
        severity TEXT NOT NULL DEFAULT 'info',
        outcome TEXT NOT NULL DEFAULT 'success',
        space_id TEXT NOT NULL DEFAULT '',
        device_id TEXT NOT NULL DEFAULT '',
        runtime_id TEXT NOT NULL DEFAULT '',
        session_id TEXT NOT NULL DEFAULT '',
        principal_type TEXT NOT NULL DEFAULT '',
        auth_method TEXT NOT NULL DEFAULT '',
        ip_address TEXT NOT NULL DEFAULT '',
        user_agent TEXT NOT NULL DEFAULT '',
        reason_code TEXT NOT NULL DEFAULT '',
        details_json TEXT NOT NULL DEFAULT '',
        occurred_at TEXT NOT NULL DEFAULT ''
    )`).Error; err != nil {
		return err
	}

	canonical := strings.TrimSpace(spaceidentity.DefaultSpaceID())
	insert := `INSERT OR REPLACE INTO security_audit_events_space_cutover
        (event_id,event_type,severity,outcome,space_id,device_id,runtime_id,session_id,principal_type,auth_method,ip_address,user_agent,reason_code,details_json,occurred_at)
        SELECT ` + eventID + `,` + pick("event_type", "''") + `,` + pick("severity", "'info'") + `,` + pick("outcome", "'success'") + `,?,` +
		pick("device_id", "''") + `,` + pick("runtime_id", "''") + `,` + pick("session_id", "''") + `,` + principal + `,` +
		pick("auth_method", "''") + `,` + pick("ip_address", "''") + `,` + pick("user_agent", "''") + `,` + pick("reason_code", "''") + `,` +
		details + `,` + occurred + ` FROM security_audit_events`
	if err := s.DB().Exec(insert, canonical).Error; err != nil {
		return fmt.Errorf("space identity cutover: copy audit events: %w", err)
	}
	if err := s.DB().Exec("DROP TABLE security_audit_events").Error; err != nil {
		return err
	}
	if err := s.DB().Exec("ALTER TABLE security_audit_events_space_cutover RENAME TO security_audit_events").Error; err != nil {
		return err
	}
	for _, sql := range []string{
		"CREATE INDEX IF NOT EXISTS idx_audit_space ON security_audit_events(space_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_device ON security_audit_events(device_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_type ON security_audit_events(event_type)",
		"CREATE INDEX IF NOT EXISTS idx_audit_time ON security_audit_events(occurred_at)",
	} {
		if err := s.DB().Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}
