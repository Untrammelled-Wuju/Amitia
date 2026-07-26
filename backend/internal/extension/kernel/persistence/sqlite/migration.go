package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

var schemaMigrations = []string{
	`CREATE TABLE IF NOT EXISTS extension_definitions (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		version TEXT NOT NULL,
		manifest_version INTEGER NOT NULL,
		definition_json TEXT NOT NULL,
		definition_hash TEXT,
		publisher_id TEXT,
		trust_level TEXT,
		source_type TEXT,
		created_at DATETIME NOT NULL,
		UNIQUE(extension_id, version)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_definitions_ext_id ON extension_definitions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_definitions_version ON extension_definitions(version)`,

	`CREATE TABLE IF NOT EXISTS extension_installations (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL UNIQUE,
		version TEXT NOT NULL,
		package_hash TEXT,
		install_path TEXT,
		installed INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 0,
		generation INTEGER NOT NULL DEFAULT 0,
		installed_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		installation_json TEXT,
		active_snapshot_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_installations_ext_id ON extension_installations(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_modules (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		module_type TEXT NOT NULL,
		runtime_type TEXT,
		entry_path TEXT,
		enabled INTEGER NOT NULL DEFAULT 1,
		definition_json TEXT NOT NULL,
		definition_hash TEXT,
		UNIQUE(extension_id, module_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_modules_ext_id ON extension_modules(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_modules_mod_id ON extension_modules(module_id)`,

	`CREATE TABLE IF NOT EXISTS extension_contributions (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		contribution_id TEXT NOT NULL,
		contribution_type TEXT NOT NULL,
		definition_json TEXT NOT NULL,
		enabled_override INTEGER,
		registered INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		UNIQUE(extension_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_contributions_ext_id ON extension_contributions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_contributions_mod_id ON extension_contributions(module_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_contributions_type ON extension_contributions(contribution_type)`,

	`CREATE TABLE IF NOT EXISTS extension_runtime_definitions (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		runtime_type TEXT NOT NULL,
		entry_point TEXT,
		definition_json TEXT NOT NULL,
		UNIQUE(extension_id, module_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_runtime_defs_ext_id ON extension_runtime_definitions(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_dependencies (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		dependency_type TEXT NOT NULL,
		dependency_id TEXT NOT NULL,
		version_required TEXT,
		optional INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_dependencies_ext_id ON extension_dependencies(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_dependencies_mod_id ON extension_dependencies(module_id)`,

	`CREATE TABLE IF NOT EXISTS extension_enablement_overrides (
		id TEXT PRIMARY KEY,
		subject_kind TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		parent_id TEXT,
		owner_id TEXT,
		enablement_state TEXT,
		desired_runtime TEXT,
		installation_state TEXT,
		definition_state TEXT,
		actual_runtime TEXT,
		health TEXT,
		circuit TEXT,
		scope_state TEXT,
		permission_state TEXT,
		dependency_ready INTEGER,
		platform_supported INTEGER,
		migration_required INTEGER,
		parent_enabled INTEGER,
		state_json TEXT,
		updated_at DATETIME NOT NULL,
		UNIQUE(subject_kind, subject_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_enablement_kind ON extension_enablement_overrides(subject_kind)`,

	`CREATE TABLE IF NOT EXISTS extension_scope_bindings (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		scope_type TEXT NOT NULL,
		scope_id TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		UNIQUE(extension_id, scope_type, scope_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_scope_bindings_ext_id ON extension_scope_bindings(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_permission_requirements (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		permission_name TEXT NOT NULL,
		reason TEXT,
		required INTEGER NOT NULL DEFAULT 0,
		scope TEXT,
		UNIQUE(extension_id, permission_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_perm_reqs_ext_id ON extension_permission_requirements(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_permission_grants (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		permission_name TEXT NOT NULL,
		state TEXT NOT NULL,
		granted_at DATETIME NOT NULL,
		expires_at DATETIME,
		UNIQUE(extension_id, permission_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_perm_grants_ext_id ON extension_permission_grants(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_runtime_desired_states (
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		desired_state TEXT NOT NULL,
		generation INTEGER NOT NULL,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (extension_id, module_id)
	)`,

	`CREATE TABLE IF NOT EXISTS extension_runtime_instances (
		instance_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		runtime_type TEXT NOT NULL,
		generation INTEGER NOT NULL,
		desired_state TEXT NOT NULL,
		actual_state TEXT NOT NULL,
		health TEXT NOT NULL,
		circuit TEXT NOT NULL,
		started_at DATETIME,
		stopped_at DATETIME,
		pid INTEGER,
		metadata_json TEXT,
		runtime_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_runtime_instances_ext_id ON extension_runtime_instances(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_operations (
		operation_id TEXT PRIMARY KEY,
		operation_type TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		status TEXT NOT NULL,
		error_code TEXT,
		error_message TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_operations_ext_id ON extension_operations(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_operations_status ON extension_operations(status)`,

	`CREATE TABLE IF NOT EXISTS extension_invocations (
		invocation_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		contribution_id TEXT NOT NULL,
		runtime_instance_id TEXT,
		status TEXT NOT NULL,
		input_hash TEXT,
		output_hash TEXT,
		error_code TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_invocations_op_id ON extension_invocations(operation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_resources (
		resource_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		reference TEXT NOT NULL,
		acquired_at DATETIME NOT NULL,
		expires_at DATETIME,
		owner_type TEXT,
		owner_id TEXT,
		metadata_json TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_resources_ext_id ON extension_resources(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_legacy_id_mappings (
		legacy_id TEXT PRIMARY KEY,
		canonical_id TEXT NOT NULL,
		mapping_type TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_extension_legacy_mappings_canonical ON extension_legacy_id_mappings(canonical_id)`,

	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`,
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL)`); err != nil {
		return fmt.Errorf("sqlite: create schema_migrations table: %w", err)
	}

	var current int
	row := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("sqlite: query schema version: %w", err)
	}

	for i := current; i < len(schemaMigrations); i++ {
		if _, err := db.ExecContext(ctx, schemaMigrations[i]); err != nil {
			return fmt.Errorf("sqlite: apply migration %d: %w", i+1, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))`, i+1); err != nil {
			return fmt.Errorf("sqlite: record migration %d: %w", i+1, err)
		}
	}

	return nil
}

func MigrationCount() int {
	return len(schemaMigrations)
}
