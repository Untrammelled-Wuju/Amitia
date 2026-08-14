package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
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

	`CREATE TABLE IF NOT EXISTS extension_task_definitions (
		task_definition_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		contribution_id TEXT,
		runtime_type TEXT NOT NULL,
		entry TEXT NOT NULL,
		definition_json TEXT NOT NULL,
		definition_hash TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(extension_id, module_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_defs_ext_id ON extension_task_definitions(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_task_runs (
		task_run_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		invocation_id TEXT,
		task_definition_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		status TEXT NOT NULL,
		priority INTEGER NOT NULL DEFAULT 0,
		input_json TEXT,
		input_hash TEXT,
		input_artifact_id TEXT,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		dependency_snapshot_id TEXT,
		runtime_instance_id TEXT,
		checkpoint_id TEXT,
		result_artifact_id TEXT,
		attempt INTEGER NOT NULL DEFAULT 1,
		max_attempts INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		queued_at DATETIME,
		started_at DATETIME,
		finished_at DATETIME,
		deadline_at DATETIME,
		cancel_requested_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		generation INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_runs_ext_id ON extension_task_runs(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_runs_status ON extension_task_runs(status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_runs_def_id ON extension_task_runs(task_definition_id)`,

	`CREATE TABLE IF NOT EXISTS extension_task_queue (
		task_run_id TEXT PRIMARY KEY,
		priority INTEGER NOT NULL DEFAULT 0,
		available_at DATETIME NOT NULL,
		lease_owner TEXT,
		lease_expires_at DATETIME,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_queue_priority ON extension_task_queue(priority DESC, available_at ASC, created_at ASC)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_queue_lease ON extension_task_queue(lease_expires_at)`,

	`CREATE TABLE IF NOT EXISTS extension_task_checkpoints (
		checkpoint_id TEXT PRIMARY KEY,
		task_run_id TEXT NOT NULL,
		checkpoint_version INTEGER NOT NULL,
		payload_json TEXT NOT NULL,
		payload_hash TEXT NOT NULL,
		definition_hash TEXT,
		input_hash TEXT,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_checkpoints_run_id ON extension_task_checkpoints(task_run_id, checkpoint_version DESC)`,

	`CREATE TABLE IF NOT EXISTS extension_task_progress (
		task_run_id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL,
		progress_json TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_progress_updated ON extension_task_progress(updated_at)`,

	`CREATE TABLE IF NOT EXISTS extension_task_results (
		task_run_id TEXT PRIMARY KEY,
		result_type TEXT NOT NULL,
		result_json TEXT,
		artifact_id TEXT,
		result_hash TEXT,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_results_artifact ON extension_task_results(artifact_id)`,

	`CREATE TABLE IF NOT EXISTS extension_task_recovery_records (
		recovery_id TEXT PRIMARY KEY,
		task_run_id TEXT NOT NULL,
		reason TEXT NOT NULL,
		checkpoint_id TEXT,
		strategy TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		resolved_at DATETIME,
		details_json TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_recovery_run_id ON extension_task_recovery_records(task_run_id)`,

	`CREATE TABLE IF NOT EXISTS extension_task_cleanup_records (
		cleanup_id TEXT PRIMARY KEY,
		task_run_id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_ref TEXT NOT NULL,
		policy TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		completed_at DATETIME,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_task_cleanup_run_id ON extension_task_cleanup_records(task_run_id)`,

	`CREATE TABLE IF NOT EXISTS extension_wasm_runtime_definitions (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		module_path TEXT NOT NULL,
		module_hash TEXT NOT NULL,
		module_sha256 TEXT NOT NULL,
		engine_type TEXT NOT NULL,
		abi TEXT NOT NULL,
		memory_limit_bytes INTEGER NOT NULL,
		fuel_limit INTEGER NOT NULL,
		instance_policy TEXT NOT NULL,
		deterministic INTEGER NOT NULL DEFAULT 0,
		entry_export TEXT NOT NULL,
		max_output_bytes INTEGER NOT NULL,
		max_host_calls INTEGER NOT NULL,
		call_timeout_ms INTEGER NOT NULL,
		allowed_imports TEXT,
		definition_hash TEXT,
		definition_version INTEGER NOT NULL DEFAULT 1,
		generation INTEGER NOT NULL DEFAULT 1,
		definition_json TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(extension_id, module_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wasm_defs_ext_id ON extension_wasm_runtime_definitions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wasm_defs_mod_id ON extension_wasm_runtime_definitions(module_id)`,

	`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL,
		checksum TEXT NOT NULL DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_points (
		hook_point_id TEXT PRIMARY KEY,
		contract_version INTEGER NOT NULL,
		description TEXT,
		definition_json TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_points_contract_ver ON extension_hook_points(contract_version)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_contributions (
		contribution_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		hook_point_id TEXT NOT NULL,
		contract_version INTEGER NOT NULL,
		phase TEXT NOT NULL,
		entry TEXT NOT NULL,
		priority INTEGER NOT NULL,
		before_json TEXT,
		after_json TEXT,
		timeout_ms INTEGER NOT NULL DEFAULT 0,
		failure_policy_json TEXT,
		mutation_claims_json TEXT,
		enabled_override INTEGER NOT NULL DEFAULT 1,
		definition_hash TEXT,
		definition_json TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE(extension_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_contribs_hook_point ON extension_hook_contributions(hook_point_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_contribs_ext_id ON extension_hook_contributions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_contribs_phase ON extension_hook_contributions(phase)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_registrations (
		registration_id TEXT PRIMARY KEY,
		hook_point_id TEXT NOT NULL,
		contribution_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		status TEXT NOT NULL,
		registered_at DATETIME NOT NULL,
		unregistered_at DATETIME,
		UNIQUE(hook_point_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_regs_hook_point ON extension_hook_registrations(hook_point_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_regs_ext_id ON extension_hook_registrations(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_invocations (
		hook_invocation_id TEXT PRIMARY KEY,
		operation_id TEXT,
		parent_invocation_id TEXT,
		contribution_id TEXT NOT NULL,
		hook_point_id TEXT,
		phase TEXT,
		sequence INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL,
		input_hash TEXT,
		result_hash TEXT,
		decision TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		runtime_instance_id TEXT,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_invocations_op_id ON extension_hook_invocations(operation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_invocations_contrib_id ON extension_hook_invocations(contribution_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_invocations_hook_point ON extension_hook_invocations(hook_point_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_invocations_started_at ON extension_hook_invocations(started_at)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_mutations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hook_invocation_id TEXT NOT NULL,
		path TEXT NOT NULL,
		operation TEXT NOT NULL,
		before_hash TEXT,
		after_hash TEXT,
		applied INTEGER NOT NULL DEFAULT 0,
		conflict INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_mutations_invocation_id ON extension_hook_mutations(hook_invocation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_conflicts (
		conflict_id TEXT PRIMARY KEY,
		hook_invocation_id TEXT NOT NULL,
		contribution_id TEXT,
		path TEXT NOT NULL,
		operation TEXT,
		conflict_mode TEXT,
		detail TEXT,
		resolved INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_conflicts_invocation_id ON extension_hook_conflicts(hook_invocation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_hook_conflicts_contribution_id ON extension_hook_conflicts(contribution_id)`,

	`CREATE TABLE IF NOT EXISTS extension_hook_circuits (
		contribution_id TEXT PRIMARY KEY,
		state TEXT NOT NULL DEFAULT 'closed',
		consecutive_fails INTEGER NOT NULL DEFAULT 0,
		total_fails INTEGER NOT NULL DEFAULT 0,
		total_success INTEGER NOT NULL DEFAULT 0,
		last_fail_code TEXT,
		last_fail_time DATETIME,
		opened_at DATETIME,
		updated_at DATETIME NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS extension_event_types (
		event_type_id TEXT NOT NULL,
		version INTEGER NOT NULL,
		description TEXT,
		payload_schema_json TEXT,
		metadata_schema_json TEXT,
		producer_policy_json TEXT NOT NULL,
		subscriber_policy_json TEXT NOT NULL,
		delivery_policy_json TEXT NOT NULL,
		ordering_policy TEXT NOT NULL,
		retention_policy_json TEXT NOT NULL,
		sensitive_fields_json TEXT,
		projection_rules_json TEXT,
		max_payload_bytes INTEGER NOT NULL,
		max_metadata_bytes INTEGER NOT NULL,
		risk_level TEXT NOT NULL DEFAULT 'low',
		definition_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (event_type_id, version)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_types_type_id ON extension_event_types(event_type_id)`,

	`CREATE TABLE IF NOT EXISTS extension_event_outbox (
		outbox_id TEXT PRIMARY KEY,
		event_id TEXT NOT NULL UNIQUE,
		event_type_id TEXT NOT NULL,
		event_version INTEGER NOT NULL,
		producer_id TEXT NOT NULL,
		producer_type TEXT NOT NULL,
		producer_generation INTEGER NOT NULL DEFAULT 0,
		event_domain TEXT NOT NULL DEFAULT '',
		causation_id TEXT NOT NULL DEFAULT '',
		aggregate_type TEXT,
		aggregate_id TEXT,
		aggregate_version INTEGER,
		partition_key TEXT,
		ordering_key TEXT,
		idempotency_key TEXT NOT NULL,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		trace_id TEXT,
		operation_id TEXT,
		parent_event_id TEXT,
		depth INTEGER NOT NULL DEFAULT 0,
		occurred_at DATETIME NOT NULL,
		published_at DATETIME,
		payload_json TEXT NOT NULL,
		metadata_json TEXT,
		payload_hash TEXT NOT NULL,
		definition_hash TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		available_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		error_code TEXT,
		error_message TEXT,
		lease_owner TEXT,
		lease_expires_at DATETIME,
		dispatched_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_outbox_status ON extension_event_outbox(status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_outbox_producer ON extension_event_outbox(producer_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_outbox_event_type ON extension_event_outbox(event_type_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_outbox_idempotency ON extension_event_outbox(idempotency_key)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_outbox_available ON extension_event_outbox(available_at)`,

	`CREATE TABLE IF NOT EXISTS extension_event_subscriptions (
		contribution_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		event_type_id TEXT NOT NULL,
		event_version_range TEXT,
		entry TEXT NOT NULL,
		filter_json TEXT,
		projection_json TEXT,
		delivery_policy_json TEXT,
		retry_policy_json TEXT,
		ordering_requirement TEXT NOT NULL DEFAULT 'none',
		timeout_ms INTEGER NOT NULL DEFAULT 5000,
		max_in_flight INTEGER NOT NULL DEFAULT 4,
		permission_requirements_json TEXT,
		scope_rule_json TEXT,
		dependency_requirements_json TEXT,
		runtime_binding_json TEXT,
		definition_hash TEXT NOT NULL,
		generation INTEGER NOT NULL DEFAULT 1,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(extension_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_subs_ext_id ON extension_event_subscriptions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_subs_type_id ON extension_event_subscriptions(event_type_id)`,

	`CREATE TABLE IF NOT EXISTS extension_event_deliveries (
		delivery_id TEXT PRIMARY KEY,
		event_id TEXT NOT NULL,
		subscription_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		partition_key TEXT,
		ordering_key TEXT,
		sequence INTEGER NOT NULL DEFAULT 0,
		attempt INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 5,
		available_at DATETIME NOT NULL,
		lease_owner TEXT,
		lease_expires_at DATETIME,
		runtime_instance_id TEXT,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		projected_payload_hash TEXT,
		started_at DATETIME,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_status ON extension_event_deliveries(status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_ext_id ON extension_event_deliveries(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_sub_id ON extension_event_deliveries(subscription_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_event_id ON extension_event_deliveries(event_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_lease ON extension_event_deliveries(lease_expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_deliveries_available ON extension_event_deliveries(available_at)`,

	`CREATE TABLE IF NOT EXISTS extension_event_dead_letters (
		dead_letter_id TEXT PRIMARY KEY,
		event_id TEXT NOT NULL,
		delivery_id TEXT NOT NULL,
		subscription_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		event_type_id TEXT NOT NULL,
		event_version INTEGER NOT NULL,
		reason TEXT NOT NULL,
		error_code TEXT,
		error_message TEXT,
		attempts INTEGER NOT NULL DEFAULT 0,
		partition_key TEXT,
		ordering_key TEXT,
		payload_hash TEXT,
		projected_payload_hash TEXT,
		definition_hash TEXT,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		runtime_instance_id TEXT,
		trace_id TEXT,
		operation_id TEXT,
		origin_event_json TEXT,
		subscription_snapshot_json TEXT,
		created_at DATETIME NOT NULL,
		replay_count INTEGER NOT NULL DEFAULT 0,
		last_replay_at DATETIME,
		status TEXT NOT NULL DEFAULT 'pending',
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_dead_letters_ext_id ON extension_event_dead_letters(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_dead_letters_sub_id ON extension_event_dead_letters(subscription_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_dead_letters_status ON extension_event_dead_letters(status)`,

	`CREATE TABLE IF NOT EXISTS extension_event_operations (
		operation_id TEXT PRIMARY KEY,
		operation_type TEXT NOT NULL,
		extension_id TEXT,
		status TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		metadata_json TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_ops_ext_id ON extension_event_operations(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_ops_status ON extension_event_operations(status)`,

	`CREATE TABLE IF NOT EXISTS extension_event_invocations (
		invocation_id TEXT PRIMARY KEY,
		operation_id TEXT,
		event_id TEXT NOT NULL,
		delivery_id TEXT,
		subscription_id TEXT,
		attempt INTEGER NOT NULL DEFAULT 0,
		runtime_instance_id TEXT,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		trace_id TEXT,
		filter_result TEXT,
		projection_result TEXT,
		ordering_result TEXT,
		permission_result TEXT,
		scope_result TEXT,
		status TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_invocations_op_id ON extension_event_invocations(operation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_invocations_event_id ON extension_event_invocations(event_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_invocations_sub_id ON extension_event_invocations(subscription_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_invocations_started ON extension_event_invocations(started_at)`,

	`CREATE TABLE IF NOT EXISTS extension_event_side_effects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		invocation_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		target TEXT NOT NULL,
		hash TEXT,
		occurred_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_side_effects_inv_id ON extension_event_side_effects(invocation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_event_audit (
		audit_id INTEGER PRIMARY KEY AUTOINCREMENT,
		operation_id TEXT,
		invocation_id TEXT,
		event_id TEXT,
		delivery_id TEXT,
		action TEXT NOT NULL,
		actor TEXT,
		extension_id TEXT,
		timestamp DATETIME NOT NULL,
		payload_hash TEXT,
		error_code TEXT,
		success INTEGER NOT NULL DEFAULT 0,
		detail_json TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_audit_event_id ON extension_event_audit(event_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_audit_ext_id ON extension_event_audit(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_audit_action ON extension_event_audit(action)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_event_audit_timestamp ON extension_event_audit(timestamp)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_definitions (
		schedule_id TEXT PRIMARY KEY,
		contribution_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		trigger_type TEXT NOT NULL,
		trigger_json TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_json TEXT NOT NULL,
		timezone TEXT NOT NULL,
		start_at DATETIME,
		end_at DATETIME,
		misfire_policy TEXT NOT NULL,
		overlap_policy TEXT NOT NULL,
		retry_policy_json TEXT NOT NULL,
		jitter_policy_json TEXT NOT NULL,
		concurrency_policy_json TEXT NOT NULL,
		permission_requirements_json TEXT,
		scope_rule_json TEXT,
		dependency_requirements_json TEXT,
		dst_spring_policy TEXT DEFAULT 'skip',
		dst_fall_policy TEXT DEFAULT 'fire_once_first',
		execution_owner TEXT NOT NULL DEFAULT 'backend',
		definition_hash TEXT NOT NULL,
		version TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_defs_ext_id ON extension_schedule_definitions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_defs_contrib_id ON extension_schedule_definitions(contribution_id)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_states (
		schedule_id TEXT PRIMARY KEY,
		enabled INTEGER NOT NULL DEFAULT 0,
		paused INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'created',
		last_scheduled_at DATETIME,
		last_triggered_at DATETIME,
		last_finished_at DATETIME,
		next_scheduled_at DATETIME,
		next_effective_at DATETIME,
		last_result TEXT,
		failure_count INTEGER NOT NULL DEFAULT 0,
		generation INTEGER NOT NULL DEFAULT 1,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_states_status ON extension_schedule_states(status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_states_next_effective ON extension_schedule_states(next_effective_at)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_triggers (
		trigger_id TEXT PRIMARY KEY,
		schedule_id TEXT NOT NULL,
		scheduled_at DATETIME NOT NULL,
		effective_at DATETIME NOT NULL,
		triggered_at DATETIME,
		idempotency_key TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'waiting',
		lease_owner TEXT,
		lease_expires_at DATETIME,
		scope_snapshot_id TEXT,
		permission_snapshot_id TEXT,
		dependency_snapshot_id TEXT,
		operation_id TEXT,
		invocation_id TEXT,
		attempt INTEGER NOT NULL DEFAULT 0,
		generation INTEGER NOT NULL DEFAULT 1,
		manual INTEGER NOT NULL DEFAULT 0,
		error_code TEXT,
		error_message TEXT,
		jitter_applied_ms INTEGER NOT NULL DEFAULT 0,
		misfire_decision TEXT,
		overlap_decision TEXT,
		dst_decision TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(schedule_id, scheduled_at, generation)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_triggers_sched_id ON extension_schedule_triggers(schedule_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_triggers_status ON extension_schedule_triggers(status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_triggers_effective ON extension_schedule_triggers(effective_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_triggers_idempotency ON extension_schedule_triggers(idempotency_key)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_triggers_lease ON extension_schedule_triggers(lease_expires_at)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_runs (
		run_id TEXT PRIMARY KEY,
		trigger_id TEXT NOT NULL,
		schedule_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'running',
		attempt INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		operation_id TEXT,
		invocation_id TEXT,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		result_json TEXT,
		error_code TEXT,
		error_message TEXT,
		generation INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_runs_sched_id ON extension_schedule_runs(schedule_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_runs_trigger_id ON extension_schedule_runs(trigger_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_runs_status ON extension_schedule_runs(status)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_leases (
		lease_id TEXT PRIMARY KEY,
		trigger_id TEXT NOT NULL,
		schedule_id TEXT NOT NULL,
		lease_owner TEXT NOT NULL,
		acquired_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		released INTEGER NOT NULL DEFAULT 0,
		released_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_leases_trigger_id ON extension_schedule_leases(trigger_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_leases_sched_id ON extension_schedule_leases(schedule_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_leases_expires ON extension_schedule_leases(expires_at)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_misfires (
		misfire_id TEXT PRIMARY KEY,
		schedule_id TEXT NOT NULL,
		scheduled_at DATETIME NOT NULL,
		detected_at DATETIME NOT NULL,
		policy TEXT NOT NULL,
		action TEXT NOT NULL,
		skipped_count INTEGER NOT NULL DEFAULT 0,
		detail TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_misfires_sched_id ON extension_schedule_misfires(schedule_id)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_retries (
		retry_id TEXT PRIMARY KEY,
		trigger_id TEXT NOT NULL,
		schedule_id TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		max_attempts INTEGER NOT NULL,
		error_code TEXT NOT NULL,
		backoff_ms INTEGER NOT NULL,
		available_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_retries_trigger_id ON extension_schedule_retries(trigger_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_retries_sched_id ON extension_schedule_retries(schedule_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_retries_available ON extension_schedule_retries(available_at)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_circuits (
		schedule_id TEXT PRIMARY KEY,
		state TEXT NOT NULL DEFAULT 'closed',
		consecutive_fails INTEGER NOT NULL DEFAULT 0,
		total_fails INTEGER NOT NULL DEFAULT 0,
		total_success INTEGER NOT NULL DEFAULT 0,
		last_fail_code TEXT,
		last_fail_time DATETIME,
		opened_at DATETIME,
		updated_at DATETIME NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS extension_schedule_quarantines (
		quarantine_id TEXT PRIMARY KEY,
		schedule_id TEXT NOT NULL,
		reason TEXT NOT NULL,
		detail TEXT,
		quarantined_at DATETIME NOT NULL,
		released_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_sched_quarantines_sched_id ON extension_schedule_quarantines(schedule_id)`,

	`CREATE TABLE IF NOT EXISTS extension_ui_contributions (
		contribution_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		slot_id TEXT NOT NULL,
		contract_version INTEGER NOT NULL DEFAULT 0,
		definition_json TEXT NOT NULL,
		enabled_override INTEGER,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(extension_id, contribution_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_ui_contribs_ext_id ON extension_ui_contributions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_ui_contribs_slot_id ON extension_ui_contributions(slot_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_ui_contribs_kind ON extension_ui_contributions(kind)`,

	`CREATE TABLE IF NOT EXISTS extension_page_sessions (
		session_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		page_id TEXT NOT NULL,
		state TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		last_active_at DATETIME NOT NULL,
		scope_snapshot TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_page_sessions_ext_id ON extension_page_sessions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_page_sessions_state ON extension_page_sessions(state)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_page_sessions_last_active ON extension_page_sessions(last_active_at)`,

	`CREATE TABLE IF NOT EXISTS extension_migration_definitions (
		migration_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		from_version_range TEXT NOT NULL,
		to_version TEXT NOT NULL,
		entry TEXT NOT NULL,
		runtime_type TEXT NOT NULL,
		direction TEXT NOT NULL,
		data_domains_json TEXT,
		input_schema TEXT,
		output_schema TEXT,
		checkpoint_schema TEXT,
		idempotency TEXT NOT NULL DEFAULT 'idempotent',
		reversibility TEXT NOT NULL DEFAULT 'fully_reversible',
		forward_migration_id TEXT,
		reverse_migration_id TEXT,
		precondition_json TEXT,
		postcondition_json TEXT,
		permission_reqs_json TEXT,
		scope_rule_json TEXT,
		resource_limits_json TEXT,
		definition_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_defs_ext_id ON extension_migration_definitions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_defs_to_ver ON extension_migration_definitions(to_version)`,

	`CREATE TABLE IF NOT EXISTS extension_migration_operations (
		operation_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		from_version TEXT NOT NULL,
		to_version TEXT NOT NULL,
		from_definition_hash TEXT,
		to_definition_hash TEXT,
		migration_path_json TEXT,
		snapshot_id TEXT,
		status TEXT NOT NULL DEFAULT 'created',
		current_step INTEGER NOT NULL DEFAULT 0,
		checkpoint_id TEXT,
		task_run_id TEXT,
		reversibility TEXT NOT NULL DEFAULT 'fully_reversible',
		requires_user_confirm INTEGER NOT NULL DEFAULT 0,
		user_confirmed INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_ops_ext_id ON extension_migration_operations(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_ops_status ON extension_migration_operations(status)`,

	`CREATE TABLE IF NOT EXISTS extension_migration_steps (
		id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_id INTEGER NOT NULL,
		migration_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		input_hash TEXT,
		output_hash TEXT,
		checkpoint_id TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		UNIQUE(operation_id, step_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_steps_op_id ON extension_migration_steps(operation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_migration_checkpoints (
		checkpoint_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_id INTEGER NOT NULL,
		migration_id TEXT NOT NULL,
		stage TEXT NOT NULL,
		cursor_json TEXT,
		batch_number INTEGER NOT NULL DEFAULT 0,
		processed_count INTEGER NOT NULL DEFAULT 0,
		input_hash TEXT,
		definition_hash TEXT,
		snapshot_id TEXT,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_mig_ckpts_op_id ON extension_migration_checkpoints(operation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_data_snapshots (
		snapshot_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		operation_id TEXT,
		generation INTEGER NOT NULL DEFAULT 0,
		total_bytes INTEGER NOT NULL DEFAULT 0,
		manifest_hash TEXT NOT NULL,
		retention_policy TEXT NOT NULL DEFAULT 'until_override',
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_snapshots_ext_id ON extension_data_snapshots(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_snapshots_op_id ON extension_data_snapshots(operation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_snapshot_entries (
		entry_id TEXT PRIMARY KEY,
		snapshot_id TEXT NOT NULL,
		entry_type TEXT NOT NULL,
		source_path TEXT NOT NULL,
		snap_path TEXT NOT NULL,
		size_bytes INTEGER NOT NULL DEFAULT 0,
		hash TEXT NOT NULL,
		page_count INTEGER NOT NULL DEFAULT 0,
		wal_handled INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_snap_entries_snap_id ON extension_snapshot_entries(snapshot_id)`,

	`CREATE TABLE IF NOT EXISTS extension_canary_policies (
		policy_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		mode TEXT NOT NULL,
		stages_json TEXT NOT NULL,
		cohort_key TEXT NOT NULL DEFAULT 'character',
		stable_seed TEXT NOT NULL,
		min_observations INTEGER NOT NULL DEFAULT 10,
		min_duration_sec INTEGER NOT NULL DEFAULT 60,
		max_duration_sec INTEGER NOT NULL DEFAULT 3600,
		health_policy_json TEXT,
		abort_policy_json TEXT,
		write_strategy TEXT NOT NULL DEFAULT 'old_only',
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_pol_ext_id ON extension_canary_policies(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_canary_stages (
		stage_id TEXT PRIMARY KEY,
		policy_id TEXT NOT NULL,
		mode TEXT NOT NULL,
		percentage INTEGER NOT NULL DEFAULT 0,
		character_ids_json TEXT,
		conversation_ids_json TEXT,
		contribution_ids_json TEXT,
		min_duration_sec INTEGER NOT NULL DEFAULT 0,
		min_invocations INTEGER NOT NULL DEFAULT 0,
		auto_advance INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_stages_pol_id ON extension_canary_stages(policy_id)`,

	`CREATE TABLE IF NOT EXISTS extension_canary_assignments (
		assignment_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		contribution_id TEXT,
		cohort_type TEXT NOT NULL,
		cohort_id TEXT NOT NULL,
		generation INTEGER NOT NULL,
		stage_id TEXT,
		assigned_at DATETIME NOT NULL,
		assignment_hash TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_assign_ext_id ON extension_canary_assignments(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_assign_cohort ON extension_canary_assignments(cohort_type, cohort_id)`,

	`CREATE TABLE IF NOT EXISTS extension_canary_metrics (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		generation INTEGER NOT NULL,
		stage_id TEXT,
		metric_name TEXT NOT NULL,
		metric_value REAL NOT NULL DEFAULT 0,
		sample_count INTEGER NOT NULL DEFAULT 0,
		window_start DATETIME NOT NULL,
		window_end DATETIME NOT NULL,
		baseline_value REAL NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'normal'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_metrics_ext_gen ON extension_canary_metrics(extension_id, generation)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_metrics_window ON extension_canary_metrics(window_start, window_end)`,

	`CREATE TABLE IF NOT EXISTS extension_generation_routes (
		id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		contribution_id TEXT,
		cohort_type TEXT NOT NULL,
		cohort_id TEXT NOT NULL,
		generation INTEGER NOT NULL,
		stage_id TEXT,
		reason TEXT NOT NULL DEFAULT 'fallback',
		assigned_at DATETIME NOT NULL,
		UNIQUE(extension_id, cohort_type, cohort_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_gen_routes_ext_cohort ON extension_generation_routes(extension_id, cohort_type, cohort_id)`,

	`CREATE TABLE IF NOT EXISTS extension_canary_states (
		canary_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		policy_id TEXT NOT NULL,
		old_generation INTEGER NOT NULL,
		new_generation INTEGER NOT NULL,
		status TEXT NOT NULL,
		current_stage INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		paused_at DATETIME,
		finished_at DATETIME,
		abort_reason TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_canary_states_ext_id ON extension_canary_states(extension_id)`,

	`CREATE TABLE IF NOT EXISTS extension_rollback_plans (
		rollback_id TEXT PRIMARY KEY,
		operation_id TEXT,
		extension_id TEXT NOT NULL,
		from_generation INTEGER NOT NULL,
		to_generation INTEGER NOT NULL,
		level TEXT NOT NULL,
		plan_json TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'created',
		automatic INTEGER NOT NULL DEFAULT 0,
		requires_user_action INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_rollback_ext_id ON extension_rollback_plans(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_rollback_status ON extension_rollback_plans(status)`,

	`CREATE TABLE IF NOT EXISTS extension_rollback_steps (
		id TEXT PRIMARY KEY,
		rollback_id TEXT NOT NULL,
		step_id INTEGER NOT NULL,
		step_type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		error_code TEXT,
		error_message TEXT,
		UNIQUE(rollback_id, step_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_rollback_steps_rb_id ON extension_rollback_steps(rollback_id)`,

	`CREATE TABLE IF NOT EXISTS extension_lifecycle_journal (
		entry_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_id TEXT NOT NULL,
		step_type TEXT NOT NULL,
		status TEXT NOT NULL,
		input_hash TEXT,
		output_hash TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		compensation_json TEXT,
		error_code TEXT,
		error_message TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_journal_op_id ON extension_lifecycle_journal(operation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_journal_status ON extension_lifecycle_journal(status)`,

	`CREATE TABLE IF NOT EXISTS extension_side_effect_assessments (
		id TEXT PRIMARY KEY,
		rollback_id TEXT,
		extension_id TEXT NOT NULL,
		contribution_id TEXT,
		side_effect_class TEXT NOT NULL,
		reversibility TEXT NOT NULL,
		can_compensate INTEGER NOT NULL DEFAULT 0,
		compensation_action TEXT,
		evidence TEXT,
		assessed_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_se_assess_rb_id ON extension_side_effect_assessments(rollback_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_se_assess_ext_id ON extension_side_effect_assessments(extension_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_permission_grants (
		grant_id TEXT PRIMARY KEY,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		permission_id TEXT NOT NULL,
		scope_type TEXT NOT NULL,
		scope_id TEXT NOT NULL,
		scope_data TEXT,
		decision TEXT NOT NULL,
		input_binding TEXT,
		target_binding TEXT,
		issued_at DATETIME NOT NULL,
		expires_at DATETIME,
		issued_by TEXT NOT NULL,
		reason TEXT,
		revoked_at DATETIME,
		manifest_ver TEXT,
		UNIQUE(subject_type, subject_id, permission_id, scope_type, scope_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_perm_grants_subject ON kernel_permission_grants(subject_type, subject_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_perm_grants_perm ON kernel_permission_grants(permission_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_perm_grants_active ON kernel_permission_grants(revoked_at)`,

	`CREATE TABLE IF NOT EXISTS kernel_scope_bindings (
		binding_id TEXT PRIMARY KEY,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		scope_type TEXT NOT NULL,
		scope_id TEXT NOT NULL,
		character_id TEXT,
		conversation_id TEXT,
		extension_id TEXT,
		module_id TEXT,
		resource_type TEXT,
		resource_id TEXT,
		invocation_id TEXT,
		session_id TEXT,
		state TEXT NOT NULL DEFAULT 'active',
		source TEXT NOT NULL DEFAULT 'system',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		expires_at DATETIME,
		metadata TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_scope_bindings_subject ON kernel_scope_bindings(subject_type, subject_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_scope_bindings_scope ON kernel_scope_bindings(scope_type, scope_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_scope_bindings_state ON kernel_scope_bindings(state)`,

	`CREATE TABLE IF NOT EXISTS kernel_scope_snapshots (
		snapshot_id TEXT PRIMARY KEY,
		invocation_id TEXT NOT NULL,
		resolved_scopes TEXT NOT NULL,
		character_id TEXT,
		conversation_id TEXT,
		extension_id TEXT,
		module_id TEXT,
		created_at DATETIME NOT NULL,
		expires_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_scope_snapshots_inv ON kernel_scope_snapshots(invocation_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_dev_workspaces (
		workspace_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL UNIQUE,
		path TEXT NOT NULL,
		manifest_path TEXT NOT NULL,
		current_revision TEXT,
		status TEXT NOT NULL DEFAULT 'registered',
		watch_enabled INTEGER NOT NULL DEFAULT 0,
		auto_reload INTEGER NOT NULL DEFAULT 0,
		dev_trust_granted INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_dev_workspaces_ext_id ON kernel_dev_workspaces(extension_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_dev_revisions (
		revision_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		manifest_hash TEXT NOT NULL,
		source_hash TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'building',
		build_duration_ms INTEGER,
		artifact_path TEXT,
		error_message TEXT,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_dev_revisions_ws_id ON kernel_dev_revisions(workspace_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_dev_sessions (
		session_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		device_id TEXT,
		user_agent TEXT,
		started_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		revoked INTEGER NOT NULL DEFAULT 0,
		revoked_at DATETIME,
		scope TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_dev_sessions_ws_id ON kernel_dev_sessions(workspace_id)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_sessions (
		session_id TEXT PRIMARY KEY,
		contribution_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		generation INTEGER NOT NULL,
		slot_id TEXT NOT NULL,
		origin TEXT NOT NULL,
		nonce TEXT NOT NULL,
		state TEXT NOT NULL DEFAULT 'creating',
		sandbox_type TEXT NOT NULL DEFAULT 'web_restricted',
		csp TEXT,
		allowed_actions TEXT,
		allowed_data_sources TEXT,
		theme_json TEXT,
		locale TEXT,
		created_at DATETIME NOT NULL,
		ready_at DATETIME,
		last_active_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		closed_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_sessions_ext_id ON extension_webui_sessions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_sessions_state ON extension_webui_sessions(state)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_audit_logs (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		method TEXT NOT NULL,
		success INTEGER NOT NULL DEFAULT 0,
		error TEXT,
		bytes_in INTEGER NOT NULL DEFAULT 0,
		bytes_out INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_audit_session ON extension_webui_audit_logs(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_audit_ext ON extension_webui_audit_logs(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_audit_created ON extension_webui_audit_logs(created_at)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_resource_handles (
		handle_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		path TEXT NOT NULL,
		mime TEXT NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		read_only INTEGER NOT NULL DEFAULT 1,
		consumed INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_rh_session ON extension_webui_resource_handles(session_id)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_subscriptions (
		subscription_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		data_source_id TEXT NOT NULL,
		active INTEGER NOT NULL DEFAULT 1,
		rate_per_minute INTEGER NOT NULL DEFAULT 10,
		last_payload TEXT,
		last_update DATETIME,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_sub_session ON extension_webui_subscriptions(session_id)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_csp_violations (
		id TEXT PRIMARY KEY,
		session_id TEXT,
		extension_id TEXT,
		violation TEXT NOT NULL,
		document_uri TEXT,
		line_number INTEGER,
		column_number INTEGER,
		source_file TEXT,
		created_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_csp_session ON extension_webui_csp_violations(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_webui_csp_created ON extension_webui_csp_violations(created_at)`,

	`CREATE TABLE IF NOT EXISTS extension_webui_circuit_breakers (
		session_id TEXT PRIMARY KEY,
		state TEXT NOT NULL DEFAULT 'closed',
		failure_count INTEGER NOT NULL DEFAULT 0,
		last_failure DATETIME,
		cooldown_seconds INTEGER NOT NULL DEFAULT 60,
		updated_at DATETIME NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS extension_kv_state (
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT,
		version INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (extension_id, module_id, key)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_kv_state_ext_id ON extension_kv_state(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_kv_state_ext_mod ON extension_kv_state(extension_id, module_id)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_definitions (
		workflow_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT,
		name TEXT NOT NULL,
		description TEXT,
		schema_version TEXT NOT NULL DEFAULT '1.0',
		version TEXT NOT NULL DEFAULT '1.0.0',
		input_schema_json TEXT NOT NULL DEFAULT '{}',
		output_schema_json TEXT NOT NULL DEFAULT '{}',
		nodes_json TEXT NOT NULL DEFAULT '[]',
		permissions_json TEXT NOT NULL DEFAULT '[]',
		scope TEXT NOT NULL DEFAULT '',
		callable_by_agent INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		has_side_effects INTEGER NOT NULL DEFAULT 0,
		idempotent INTEGER NOT NULL DEFAULT 0,
		limits_json TEXT NOT NULL DEFAULT '{}',
		source TEXT NOT NULL DEFAULT 'manual',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		definition_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_defs_ext_id ON extension_workflow_definitions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_defs_enabled ON extension_workflow_definitions(enabled)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_executions (
		execution_id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'running',
		input_json TEXT,
		output_json TEXT,
		error_message TEXT,
		extension_id TEXT,
		character_id TEXT,
		conversation_id TEXT,
		operation_id TEXT,
		steps_json TEXT NOT NULL DEFAULT '[]',
		compensation_json TEXT,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_exec_wf_id ON extension_workflow_executions(workflow_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_exec_status ON extension_workflow_executions(status)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_checkpoints (
		checkpoint_id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		execution_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		input_json TEXT,
		output_json TEXT,
		completed_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE(execution_id, node_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_ckpts_exec_id ON extension_workflow_checkpoints(execution_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_ckpts_wf_id ON extension_workflow_checkpoints(workflow_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_legacy_package_migrations (
		legacy_extension_id TEXT PRIMARY KEY,
		legacy_version TEXT NOT NULL,
		status TEXT NOT NULL,
		reason TEXT NOT NULL DEFAULT '',
		migrated_at DATETIME,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_legacy_package_migrations_status ON kernel_legacy_package_migrations(status)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_step_runs (
		execution_id TEXT NOT NULL,
		workflow_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		status TEXT NOT NULL,
		input_json TEXT,
		output_json TEXT,
		error_message TEXT,
		attempt INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY (execution_id, node_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_step_runs_workflow ON extension_workflow_step_runs(workflow_id, execution_id)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_trigger_bindings (
		binding_id TEXT PRIMARY KEY,
		trigger_type TEXT NOT NULL,
		event_type TEXT NOT NULL DEFAULT '',
		schedule_id TEXT NOT NULL DEFAULT '',
		workflow_id TEXT NOT NULL,
		input_json TEXT NOT NULL DEFAULT '{}',
		generation INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_wf_trigger_lookup ON extension_workflow_trigger_bindings(trigger_type, event_type, schedule_id, enabled)`,

	`CREATE TABLE IF NOT EXISTS extension_workflow_compensations (
		execution_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT NOT NULL DEFAULT '',
		executed_at DATETIME NOT NULL,
		PRIMARY KEY (execution_id, node_id)
	)`,

	`CREATE TABLE IF NOT EXISTS kernel_candidate_contributions (
		candidate_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		instance_ids_json TEXT NOT NULL DEFAULT '[]',
		generation_id TEXT NOT NULL,
		candidate_generation INTEGER NOT NULL,
		expected_stable_generation INTEGER NOT NULL DEFAULT 0,
		contribs_json TEXT NOT NULL DEFAULT '[]',
		schedule_ids_json TEXT NOT NULL DEFAULT '[]',
		artifact_path TEXT NOT NULL DEFAULT '',
		definition_hash TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'registered',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_candidate_ext_id ON kernel_candidate_contributions(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_candidate_status ON kernel_candidate_contributions(status)`,

	`CREATE TABLE IF NOT EXISTS kernel_permission_snapshots (
		snapshot_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL DEFAULT '',
		extension_id TEXT NOT NULL DEFAULT '',
		module_id TEXT NOT NULL DEFAULT '',
		generation INTEGER NOT NULL DEFAULT 0,
		character_id TEXT,
		conversation_id TEXT,
		resource_ids TEXT NOT NULL DEFAULT '[]',
		granted_perms TEXT NOT NULL DEFAULT '[]',
		granted_scopes TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		expires_at DATETIME,
		revoked_at DATETIME,
		execution_placement TEXT NOT NULL DEFAULT '',
		execution_user_id TEXT NOT NULL DEFAULT '',
		execution_device_id TEXT NOT NULL DEFAULT '',
		execution_runtime_id TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		provider_instance_id TEXT NOT NULL DEFAULT '',
		execution_binding_key TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_perm_snaps_session ON kernel_permission_snapshots(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_perm_snaps_ext_id ON kernel_permission_snapshots(extension_id)`,

	`CREATE TABLE IF NOT EXISTS host_api_audit_logs (
		call_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL DEFAULT '',
		operation_id TEXT NOT NULL DEFAULT '',
		invocation_id TEXT NOT NULL DEFAULT '',
		attempt_id TEXT NOT NULL DEFAULT '',
		extension_id TEXT NOT NULL DEFAULT '',
		module_id TEXT NOT NULL DEFAULT '',
		method TEXT NOT NULL DEFAULT '',
		generation INTEGER NOT NULL DEFAULT 0,
		permission_snapshot_id TEXT NOT NULL DEFAULT '',
		scope_snapshot_id TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		finished_at DATETIME,
		result TEXT NOT NULL DEFAULT '',
		error_code TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		side_effect TEXT NOT NULL DEFAULT '',
		input_masked TEXT NOT NULL DEFAULT '',
		phase TEXT NOT NULL DEFAULT 'end'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_host_api_audit_ext_id ON host_api_audit_logs(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_host_api_audit_method ON host_api_audit_logs(method)`,
	`CREATE INDEX IF NOT EXISTS idx_host_api_audit_result ON host_api_audit_logs(result)`,
	`CREATE INDEX IF NOT EXISTS idx_host_api_audit_trace_id ON host_api_audit_logs(trace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_host_api_audit_started_at ON host_api_audit_logs(started_at)`,

	`CREATE TABLE IF NOT EXISTS kernel_reload_cleanup_failures (
		failure_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		extension_id TEXT NOT NULL DEFAULT '',
		old_instance_id TEXT NOT NULL,
		old_generation INTEGER NOT NULL DEFAULT 0,
		new_instance_id TEXT NOT NULL DEFAULT '',
		new_generation INTEGER NOT NULL DEFAULT 0,
		error_code TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 5,
		next_retry_at TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_reload_cleanup_ws_id ON kernel_reload_cleanup_failures(workspace_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_reload_cleanup_status ON kernel_reload_cleanup_failures(status)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_reload_cleanup_next_retry ON kernel_reload_cleanup_failures(next_retry_at)`,

	`CREATE TABLE IF NOT EXISTS kernel_candidate_stable_snapshots (
		snapshot_id TEXT PRIMARY KEY,
		candidate_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		contribution_id TEXT NOT NULL DEFAULT '',
		contribution_kind TEXT NOT NULL DEFAULT '',
		stable_generation INTEGER NOT NULL DEFAULT 0,
		stable_definition_json TEXT NOT NULL DEFAULT '{}',
		stable_definition_hash TEXT NOT NULL DEFAULT '',
		stable_runtime_binding_json TEXT NOT NULL DEFAULT '{}',
		stable_permission_json TEXT NOT NULL DEFAULT '[]',
		stable_scope_json TEXT NOT NULL DEFAULT '{}',
		enablement_state TEXT NOT NULL DEFAULT '',
		generation_id TEXT NOT NULL DEFAULT '',
		captured_at TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_cand_snap_cand_id ON kernel_candidate_stable_snapshots(candidate_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_cand_snap_ext_id ON kernel_candidate_stable_snapshots(extension_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_generation_states (
		generation_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		version TEXT NOT NULL DEFAULT '',
		generation_num INTEGER NOT NULL DEFAULT 0,
		state TEXT NOT NULL DEFAULT 'preparing',
		definition_hash TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		activated_at TEXT,
		stopped_at TEXT,
		invocations INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_gen_states_ext_id ON kernel_generation_states(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_gen_states_state ON kernel_generation_states(state)`,

	`CREATE TABLE IF NOT EXISTS kernel_runtime_cleanup_tasks (
		cleanup_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		module_id TEXT NOT NULL DEFAULT '',
		old_generation INTEGER NOT NULL DEFAULT 0,
		runtime_definition_id TEXT NOT NULL DEFAULT '',
		runtime_instance_id TEXT NOT NULL DEFAULT '',
		runtime_type TEXT NOT NULL DEFAULT '',
		process_id INTEGER NOT NULL DEFAULT 0,
		cleanup_state TEXT NOT NULL DEFAULT 'pending',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error_code TEXT NOT NULL DEFAULT '',
		last_error_message TEXT NOT NULL DEFAULT '',
		next_retry_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_rt_cleanup_ext_id ON kernel_runtime_cleanup_tasks(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_rt_cleanup_state ON kernel_runtime_cleanup_tasks(cleanup_state)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_rt_cleanup_instance ON kernel_runtime_cleanup_tasks(runtime_instance_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_lifecycle_operations (
		operation_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		operation_type TEXT NOT NULL,
		from_state TEXT NOT NULL DEFAULT '',
		target_state TEXT NOT NULL DEFAULT '',
		stable_generation INTEGER NOT NULL DEFAULT 0,
		candidate_generation INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		current_step TEXT NOT NULL DEFAULT '',
		error_code TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_lc_ops_ext_id ON kernel_lifecycle_operations(extension_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_lc_ops_status ON kernel_lifecycle_operations(status)`,

	`CREATE TABLE IF NOT EXISTS kernel_lifecycle_steps (
		step_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		result_json TEXT NOT NULL DEFAULT '{}',
		error_code TEXT NOT NULL DEFAULT '',
		started_at TEXT,
		finished_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_lc_steps_op_id ON kernel_lifecycle_steps(operation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_lc_steps_status ON kernel_lifecycle_steps(status)`,

	`CREATE TABLE IF NOT EXISTS kernel_lifecycle_compensations (
		compensation_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_name TEXT NOT NULL,
		compensation_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		error_code TEXT NOT NULL DEFAULT '',
		started_at TEXT,
		finished_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_lc_comp_op_id ON kernel_lifecycle_compensations(operation_id)`,

	`CREATE TABLE IF NOT EXISTS kernel_host_registry (
		host_client_id TEXT PRIMARY KEY,
		host_session_id TEXT NOT NULL,
		user_id TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		device_id TEXT NOT NULL DEFAULT '',
		runtime_id TEXT NOT NULL DEFAULT '',
		window_id TEXT NOT NULL DEFAULT '',
		capabilities TEXT NOT NULL DEFAULT '[]',
		entry_kind TEXT NOT NULL DEFAULT 'ui_host',
		authenticated_at TEXT NOT NULL,
		last_heartbeat TEXT NOT NULL,
		connection_state TEXT NOT NULL DEFAULT 'disconnected',
		session_token TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		expires_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_user_id ON kernel_host_registry(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_session ON kernel_host_registry(host_session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_state ON kernel_host_registry(connection_state)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_device ON kernel_host_registry(user_id, device_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_runtime ON kernel_host_registry(user_id, device_id, runtime_id)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_host_reg_kind_state ON kernel_host_registry(entry_kind, connection_state)`,

	`CREATE TABLE IF NOT EXISTS kernel_final_gate_metrics (
		id TEXT PRIMARY KEY,
		metric_name TEXT NOT NULL,
		resource_id TEXT NOT NULL DEFAULT '',
		extension_id TEXT NOT NULL DEFAULT '',
		generation INTEGER NOT NULL DEFAULT 0,
		detected_at TEXT NOT NULL,
		resolved_at TEXT,
		status TEXT NOT NULL DEFAULT 'open',
		detail TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_fg_metrics_name ON kernel_final_gate_metrics(metric_name)`,
	`CREATE INDEX IF NOT EXISTS idx_kernel_fg_metrics_status ON kernel_final_gate_metrics(status)`,

	`CREATE TABLE IF NOT EXISTS kernel_legacy_call_counters (
		metric_name TEXT PRIMARY KEY,
		count INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS extension_package_artifacts (
		artifact_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		version TEXT NOT NULL,
		archive_hash TEXT NOT NULL,
		manifest_hash TEXT NOT NULL,
		content_tree_hash TEXT NOT NULL,
		artifact_hash TEXT NOT NULL DEFAULT '',
		archive_path TEXT NOT NULL,
		installed_path TEXT NOT NULL DEFAULT '',
		size_bytes INTEGER NOT NULL,
		signature_status TEXT NOT NULL,
		signer_key_id TEXT NOT NULL DEFAULT '',
		publisher_id TEXT NOT NULL DEFAULT '',
		trust_decision TEXT NOT NULL,
		verification_report_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL,
		verified_at TEXT NOT NULL DEFAULT '',
		quarantined_at TEXT NOT NULL DEFAULT '',
		UNIQUE(extension_id, version, archive_hash)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_artifacts_extension ON extension_package_artifacts(extension_id, version)`,
	`CREATE TABLE IF NOT EXISTS extension_package_preview_sessions (
		session_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		scope_type TEXT NOT NULL,
		scope_id TEXT NOT NULL DEFAULT '',
		artifact_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		version TEXT NOT NULL,
		status TEXT NOT NULL,
		archive_hash TEXT NOT NULL,
		manifest_hash TEXT NOT NULL,
		content_tree_hash TEXT NOT NULL,
		risk_flags_json TEXT NOT NULL,
		required_confirmations_json TEXT NOT NULL,
		dependency_result_json TEXT NOT NULL,
		preview_result_json TEXT NOT NULL,
		verification_report_json TEXT NOT NULL,
		policy_version TEXT NOT NULL,
		verified_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		consumed_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_preview_owner ON extension_package_preview_sessions(user_id, scope_type, scope_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_preview_expiry ON extension_package_preview_sessions(expires_at, status)`,
	`CREATE TABLE IF NOT EXISTS extension_package_operations (
		operation_id TEXT PRIMARY KEY,
		trace_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		scope_type TEXT NOT NULL,
		scope_id TEXT NOT NULL DEFAULT '',
		extension_id TEXT NOT NULL,
		target_version TEXT NOT NULL DEFAULT '',
		operation_type TEXT NOT NULL,
		status TEXT NOT NULL,
		current_step TEXT NOT NULL,
		artifact_id TEXT NOT NULL DEFAULT '',
		preview_session_id TEXT NOT NULL DEFAULT '',
		confirmations_json TEXT NOT NULL DEFAULT '{}',
		error_code TEXT NOT NULL DEFAULT '',
		error_detail TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		completed_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_operations_owner ON extension_package_operations(user_id, started_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_operations_status ON extension_package_operations(status, updated_at)`,
	`CREATE TABLE IF NOT EXISTS extension_package_operation_steps (
		step_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		step_name TEXT NOT NULL,
		step_order INTEGER NOT NULL,
		status TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		result_json TEXT NOT NULL DEFAULT '{}',
		error_code TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT '',
		UNIQUE(operation_id, step_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_steps_operation ON extension_package_operation_steps(operation_id, step_order)`,
	`CREATE TABLE IF NOT EXISTS extension_package_rollback_points (
		rollback_point_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		source_version TEXT NOT NULL,
		source_generation INTEGER NOT NULL,
		source_version_id TEXT NOT NULL DEFAULT '',
		source_generation_id TEXT NOT NULL DEFAULT '',
		snapshot_id TEXT NOT NULL DEFAULT '',
		artifact_id TEXT NOT NULL,
		definition_snapshot_json TEXT NOT NULL,
		module_snapshot_json TEXT NOT NULL,
		contribution_snapshot_json TEXT NOT NULL,
		permission_snapshot_json TEXT NOT NULL,
		scope_snapshot_json TEXT NOT NULL,
		config_snapshot_id TEXT NOT NULL DEFAULT '',
		config_snapshot_json TEXT NOT NULL,
		secret_refs_json TEXT NOT NULL,
		resource_snapshot_json TEXT NOT NULL,
		migration_state_snapshot_json TEXT NOT NULL,
		user_data_migration_state_json TEXT NOT NULL,
		snapshot_hash TEXT NOT NULL DEFAULT '',
		retention_state TEXT NOT NULL DEFAULT 'active',
		retention_until TEXT NOT NULL DEFAULT '',
		source_operation_id TEXT NOT NULL DEFAULT '',
		installed_path TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_rollback_extension ON extension_package_rollback_points(extension_id, created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_rollback_version_identity ON extension_package_rollback_points(extension_id, source_version_id, source_generation_id)`,
	`CREATE TABLE IF NOT EXISTS extension_publisher_keys (
		key_id TEXT PRIMARY KEY,
		fingerprint TEXT NOT NULL UNIQUE,
		public_key BLOB NOT NULL,
		publisher_id TEXT NOT NULL,
		trust_source TEXT NOT NULL,
		trust_level TEXT NOT NULL,
		key_state TEXT NOT NULL,
		trusted_at TEXT NOT NULL DEFAULT '',
		revoked_at TEXT NOT NULL DEFAULT '',
		revocation_reason TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_publisher_keys_publisher ON extension_publisher_keys(publisher_id, key_state)`,
	`CREATE TABLE IF NOT EXISTS extension_package_security_audit (
		event_id TEXT PRIMARY KEY,
		event_type TEXT NOT NULL,
		package_id TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		publisher_id TEXT NOT NULL DEFAULT '',
		report_id TEXT NOT NULL DEFAULT '',
		staging_id TEXT NOT NULL DEFAULT '',
		snapshot_id TEXT NOT NULL DEFAULT '',
		operation_id TEXT NOT NULL DEFAULT '',
		details TEXT NOT NULL DEFAULT '',
		success INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_audit_type ON extension_package_security_audit(event_type, created_at)`,
	`CREATE TABLE IF NOT EXISTS extension_package_legacy_migrations (
		extension_id TEXT PRIMARY KEY,
		migration_status TEXT NOT NULL,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		legacy_path TEXT NOT NULL DEFAULT '',
		artifact_id TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS extension_package_legacy_migration_checkpoints (
		migration_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL UNIQUE,
		source_hash TEXT NOT NULL,
		preview_hash TEXT NOT NULL DEFAULT '',
		preview_session_id TEXT NOT NULL DEFAULT '',
		artifact_id TEXT NOT NULL DEFAULT '',
		operation_id TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL,
		current_step TEXT NOT NULL,
		lease_owner TEXT NOT NULL DEFAULT '',
		fencing_token INTEGER NOT NULL DEFAULT 0,
		lease_expires_at TEXT NOT NULL DEFAULT '',
		verification_hash TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		completed_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_legacy_checkpoint_state
		ON extension_package_legacy_migration_checkpoints(state, updated_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_legacy_checkpoint_lease
		ON extension_package_legacy_migration_checkpoints(lease_expires_at, lease_owner)`,
	`CREATE TABLE IF NOT EXISTS extension_package_exports (
		export_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		artifact_id TEXT NOT NULL,
		file_name TEXT NOT NULL,
		mime_type TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_exports_owner ON extension_package_exports(user_id, extension_id, expires_at)`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN reference_count INTEGER NOT NULL DEFAULT 0 CHECK(reference_count >= 0)`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN retention_state TEXT NOT NULL DEFAULT 'active'`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN retention_until TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN last_verified_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN verification_status TEXT NOT NULL DEFAULT 'pending'`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN gc_error TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN gc_attempted_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
	`CREATE TABLE IF NOT EXISTS extension_package_artifact_references (
		reference_id TEXT PRIMARY KEY,
		artifact_id TEXT NOT NULL,
		reference_type TEXT NOT NULL,
		reference_owner_id TEXT NOT NULL,
		expires_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		released_at TEXT NOT NULL DEFAULT '',
		UNIQUE(artifact_id, reference_type, reference_owner_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_artifact_refs_active ON extension_package_artifact_references(artifact_id, released_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_artifact_refs_expiry ON extension_package_artifact_references(expires_at, released_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_artifact_gc ON extension_package_artifacts(retention_state, reference_count, retention_until, created_at)`,
	`ALTER TABLE extension_package_operations ADD COLUMN stable_generation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN target_generation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN current_pointer_json TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN stable_generation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN target_generation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN current_pointer_json TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN idempotency_key TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN request_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN from_version TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN recovery_required INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE extension_package_operations ADD COLUMN cancel_requested_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN lease_owner TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN lease_expires_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 1`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_pkg_operations_idempotency ON extension_package_operations(user_id, idempotency_key) WHERE idempotency_key <> ''`,
	`CREATE TABLE IF NOT EXISTS extension_package_operation_leases (
		extension_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		lease_owner TEXT NOT NULL,
		lease_expires_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		cas_version INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_operation_leases_expiry ON extension_package_operation_leases(lease_expires_at)`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN input_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN error_detail TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN compensation_name TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN compensation_status TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN side_effect_evidence TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operation_steps ADD COLUMN cas_version INTEGER NOT NULL DEFAULT 1`,
	`CREATE TABLE IF NOT EXISTS package_confirmation_keys (
		key_id TEXT PRIMARY KEY,
		secret_reference TEXT NOT NULL,
		algorithm TEXT NOT NULL,
		state TEXT NOT NULL,
		active_from INTEGER NOT NULL,
		expires_at INTEGER,
		created_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS package_versions (
		version_id TEXT PRIMARY KEY,
		extension_id TEXT NOT NULL,
		version TEXT NOT NULL,
		artifact_id TEXT NOT NULL,
		generation_id TEXT NOT NULL,
		manifest_hash TEXT NOT NULL,
		content_tree_hash TEXT NOT NULL,
		publisher_id TEXT,
		signing_key_id TEXT,
		version_state TEXT NOT NULL,
		installed_at INTEGER,
		retained_until INTEGER,
		created_at INTEGER NOT NULL,
		uninstalled_at TEXT NOT NULL DEFAULT '',
		UNIQUE(extension_id, version)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_package_versions_ext ON package_versions(extension_id)`,
	`ALTER TABLE extension_package_operation_leases ADD COLUMN fencing_token INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE extension_package_operations ADD COLUMN fencing_token INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE extension_package_operations ADD COLUMN owner_instance_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN blocklisted_at TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_artifacts ADD COLUMN blocklist_reason TEXT NOT NULL DEFAULT ''`,
	`CREATE TABLE IF NOT EXISTS package_consistency_findings (
		finding_id TEXT PRIMARY KEY,
		metric TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		resource_ids_json TEXT NOT NULL DEFAULT '[]',
		error_detail TEXT NOT NULL DEFAULT '',
		recommended_action TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		resolved_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pkg_consistency_findings_metric ON package_consistency_findings(metric) WHERE resolved_at = ''`,
	`CREATE TABLE IF NOT EXISTS package_quarantine_metadata (
		quarantine_id TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		generation_quarantine_path TEXT NOT NULL DEFAULT '',
		current_quarantine_path TEXT NOT NULL DEFAULT '',
		original_generation_path TEXT NOT NULL DEFAULT '',
		original_current_path TEXT NOT NULL DEFAULT '',
		tree_hash TEXT NOT NULL DEFAULT '',
		artifact_id TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL DEFAULT 'active',
		fencing_token INTEGER NOT NULL DEFAULT 0,
		release_state TEXT NOT NULL DEFAULT 'active',
		release_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		released_at TEXT NOT NULL DEFAULT '',
		snapshot_json TEXT NOT NULL DEFAULT '',
		snapshot_hash TEXT NOT NULL DEFAULT '',
		expected_generation_id TEXT NOT NULL DEFAULT '',
		expected_version_id TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pkg_quarantine_ext ON package_quarantine_metadata(extension_id) WHERE state = 'active'`,
	`CREATE INDEX IF NOT EXISTS idx_pkg_quarantine_op ON package_quarantine_metadata(operation_id)`,
	`ALTER TABLE extension_package_rollback_points ADD COLUMN forward_recovery_operation_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_rollback_points ADD COLUMN forward_recovery_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_rollback_points ADD COLUMN migration_set_diff_json TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN install_operation_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN uninstall_operation_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN installed_path TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN installed_tree_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN archive_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE package_versions ADD COLUMN version_state TEXT NOT NULL DEFAULT 'pending'`,
	`ALTER TABLE package_versions ADD COLUMN retained_until TEXT`,
	`UPDATE package_versions SET version_state = 'current' WHERE version_state = 'active'`,
	`UPDATE package_versions SET version_state = 'retained' WHERE version_state = 'inactive'`,
	`UPDATE package_versions SET version_state = 'retained' WHERE version_id NOT IN (SELECT version_id FROM package_versions pv1 WHERE pv1.version_state = 'current' AND pv1.extension_id = package_versions.extension_id ORDER BY installed_at DESC LIMIT 1) AND version_state = 'current' AND extension_id IN (SELECT extension_id FROM package_versions WHERE version_state = 'current' GROUP BY extension_id HAVING COUNT(*) > 1)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_package_versions_single_current ON package_versions(extension_id) WHERE version_state = 'current'`,
	`ALTER TABLE extension_package_operations ADD COLUMN final_fencing_token INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE extension_package_operations ADD COLUMN completion_proof_hash TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN finalization_state TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE extension_package_operations ADD COLUMN journal_schema_version INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE package_versions ADD COLUMN uninstalled_at TEXT NOT NULL DEFAULT ''`,
	`CREATE TABLE IF NOT EXISTS extension_package_user_data_restore_journal (
		journal_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		table_name TEXT NOT NULL,
		total_rows INTEGER NOT NULL DEFAULT 0,
		imported_rows INTEGER NOT NULL DEFAULT 0,
		applied_count INTEGER NOT NULL DEFAULT 0,
		cursor TEXT NOT NULL DEFAULT '',
		batch_hash TEXT NOT NULL DEFAULT '',
		batch_index INTEGER NOT NULL DEFAULT 0,
		prev_batch_hash TEXT NOT NULL DEFAULT '',
		batch_algorithm_version TEXT NOT NULL DEFAULT '',
		batch_size INTEGER NOT NULL DEFAULT 0,
		namespace_hash TEXT NOT NULL DEFAULT '',
		aggregate_hash TEXT NOT NULL DEFAULT '',
		state TEXT NOT NULL DEFAULT 'pending',
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		error_detail TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (operation_id, table_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_user_data_restore_journal_op ON extension_package_user_data_restore_journal(operation_id)`,
	`CREATE TABLE IF NOT EXISTS extension_package_confirmation_nonces (
		nonce TEXT PRIMARY KEY,
		operation_id TEXT NOT NULL UNIQUE,
		operation_type TEXT NOT NULL,
		extension_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		issued_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		consumed_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_confirmation_nonces_op ON extension_package_confirmation_nonces(operation_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_pkg_confirmation_nonces_exp ON extension_package_confirmation_nonces(expires_at)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_pkg_confirmation_nonces_op_unique ON extension_package_confirmation_nonces(operation_id)`,

	`CREATE TABLE IF NOT EXISTS extension_execution_idempotency (
		idempotency_key TEXT PRIMARY KEY,
		request_fingerprint TEXT NOT NULL,
		state TEXT NOT NULL,
		work_result_json TEXT,
		safe_to_replay INTEGER NOT NULL DEFAULT 1,
		revision INTEGER NOT NULL DEFAULT 0,
		reservation_json TEXT,
		owner_instance_id TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		released_at DATETIME
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_exec_idemp_state_expires ON extension_execution_idempotency(state, expires_at)`,
	`CREATE INDEX IF NOT EXISTS idx_ext_exec_idemp_created ON extension_execution_idempotency(created_at)`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN execution_placement TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN execution_user_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN execution_device_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN execution_runtime_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN provider_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN provider_instance_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE kernel_permission_snapshots ADD COLUMN execution_binding_key TEXT NOT NULL DEFAULT ''`,
}

type dbExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL, checksum TEXT NOT NULL DEFAULT '')`); err != nil {
		return fmt.Errorf("sqlite: create schema_migrations table: %w", err)
	}

	hasChecksum, err := columnExists(ctx, db, "schema_migrations", "checksum")
	if err != nil {
		return fmt.Errorf("sqlite: check schema_migrations checksum column: %w", err)
	}
	if !hasChecksum {
		if _, err := db.ExecContext(ctx, `ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("sqlite: add checksum column to schema_migrations: %w", err)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("sqlite: query schema migrations: %w", err)
	}
	appliedChecksums := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("sqlite: scan migration row: %w", err)
		}
		appliedChecksums[version] = checksum
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("sqlite: close migration rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterate migration rows: %w", err)
	}

	if err := ensureSchemaColumns(ctx, tx); err != nil {
		return fmt.Errorf("sqlite: pre-ensure schema columns: %w", err)
	}

	for i, ddl := range schemaMigrations {
		version := i + 1
		checksum := computeChecksum(ddl)
		if existing, ok := appliedChecksums[version]; ok {
			if existing == "" {
				if _, err := tx.ExecContext(ctx, `UPDATE schema_migrations SET checksum = ? WHERE version = ?`, checksum, version); err != nil {
					return fmt.Errorf("sqlite: backfill checksum for migration %d: %w", version, err)
				}
				continue
			}
			if existing != checksum {
				if isIdempotentDDL(ddl) {
					if _, err := tx.ExecContext(ctx, ddl); err != nil {
						return fmt.Errorf("sqlite: re-verify migration %d (checksum mismatch): %w", version, err)
					}
					if _, err := tx.ExecContext(ctx, `UPDATE schema_migrations SET checksum = ? WHERE version = ?`, checksum, version); err != nil {
						return fmt.Errorf("sqlite: normalize checksum for migration %d: %w", version, err)
					}
					continue
				}
				return fmt.Errorf("sqlite: checksum mismatch for migration %d: expected %s, got %s", version, checksum, existing)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			if isAlterTableAddColumn(ddl) && isDuplicateColumnError(err) {
				// pass
			} else {
				return fmt.Errorf("sqlite: apply migration %d: %w", version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at, checksum) VALUES (?, datetime('now'), ?)`, version, checksum); err != nil {
			return fmt.Errorf("sqlite: record migration %d: %w", version, err)
		}
	}

	if err := ensureSchemaColumns(ctx, tx); err != nil {
		return fmt.Errorf("sqlite: ensure schema columns: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_wf_exec_idempotency ON extension_workflow_executions(workflow_id, idempotency_key) WHERE idempotency_key <> ''`); err != nil {
		return fmt.Errorf("sqlite: ensure workflow idempotency index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit migration transaction: %w", err)
	}
	return nil
}

func computeChecksum(ddl string) string {
	normalized := normalizeDDL(ddl)
	h := sha256.New()
	h.Write([]byte(normalized))
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeDDL(ddl string) string {
	lines := strings.Split(ddl, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed = append(trimmed, strings.TrimSpace(line))
	}
	return strings.Join(trimmed, "\n")
}

func isIdempotentDDL(ddl string) bool {
	upper := strings.ToUpper(strings.TrimSpace(ddl))
	return strings.HasPrefix(upper, "CREATE TABLE IF NOT EXISTS") ||
		strings.HasPrefix(upper, "CREATE INDEX IF NOT EXISTS") ||
		strings.HasPrefix(upper, "CREATE UNIQUE INDEX IF NOT EXISTS")
}

func isAlterTableAddColumn(ddl string) bool {
	upper := strings.ToUpper(strings.TrimSpace(ddl))
	return strings.HasPrefix(upper, "ALTER TABLE ") && strings.Contains(upper, "ADD COLUMN")
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name")
}

type columnAddition struct {
	table  string
	column string
	def    string
}

var schemaColumnAdditions = []columnAddition{
	{"extension_event_deliveries", "subscription_generation", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_event_deliveries", "target_generation", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_event_deliveries", "producer_generation", "INTEGER NOT NULL DEFAULT 0"},
	{"kernel_scope_snapshots", "generation", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_invocations", "parent_invocation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "invocation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "module_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "schedule_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "trigger_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "trace_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "idempotency_key", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "scope_snapshot_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "permission_snapshot_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "generation", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_workflow_executions", "context_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_workflow_executions", "attempt", "INTEGER NOT NULL DEFAULT 1"},
	{"extension_task_runs", "trace_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_task_runs", "correlation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_task_runs", "causation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_task_runs", "source", "TEXT NOT NULL DEFAULT ''"},
	{"extension_schedule_definitions", "execution_owner", "TEXT NOT NULL DEFAULT 'backend'"},
	{"kernel_candidate_contributions", "schedule_ids_json", "TEXT NOT NULL DEFAULT '[]'"},
	{"kernel_candidate_contributions", "expected_stable_generation", "INTEGER NOT NULL DEFAULT 0"},
	{"kernel_candidate_contributions", "promote_started_at", "TEXT NOT NULL DEFAULT ''"},
	{"kernel_candidate_contributions", "promote_committed_at", "TEXT NOT NULL DEFAULT ''"},
	{"kernel_candidate_contributions", "rollback_started_at", "TEXT NOT NULL DEFAULT ''"},
	{"kernel_candidate_contributions", "rollback_finished_at", "TEXT NOT NULL DEFAULT ''"},
	{"kernel_candidate_contributions", "module_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "config_snapshot_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "secret_refs_json", "TEXT NOT NULL DEFAULT '[]'"},
	{"extension_package_rollback_points", "resource_snapshot_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "migration_state_snapshot_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "user_data_migration_state_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "snapshot_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "retention_state", "TEXT NOT NULL DEFAULT 'active'"},
	{"extension_package_rollback_points", "retention_until", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "source_operation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "security_policy_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_operation_leases", "lease_token", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_installations", "last_operation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_installations", "current_generation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_installations", "current_artifact_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_installations", "current_version_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_installations", "last_verified_at", "TEXT NOT NULL DEFAULT ''"},
	{"extension_installations", "recovery_state", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "install_operation_id", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "uninstall_operation_id", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "uninstalled_at", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "installed_path", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "installed_tree_hash", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "archive_hash", "TEXT NOT NULL DEFAULT ''"},
	{"package_versions", "version_state", "TEXT NOT NULL DEFAULT 'pending'"},
	{"package_versions", "retained_until", "TEXT"},
	{"extension_package_operation_leases", "fencing_token", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_package_operations", "fencing_token", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_package_operations", "owner_instance_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_artifacts", "blocklisted_at", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_artifacts", "blocklist_reason", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "forward_recovery_operation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "forward_recovery_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "migration_set_diff_json", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_resource_quarantine", "size", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_package_resource_quarantine", "namespace_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_resource_quarantine", "original_path", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_resource_quarantine", "content_storage_reference", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_user_data_restore_journal", "applied_count", "INTEGER NOT NULL DEFAULT 0"},
	{"extension_package_user_data_restore_journal", "cursor", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_user_data_restore_journal", "batch_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_user_data_restore_journal", "namespace_hash", "TEXT NOT NULL DEFAULT ''"},
	{"package_quarantine_metadata", "fencing_token", "INTEGER NOT NULL DEFAULT 0"},
	{"package_quarantine_metadata", "release_state", "TEXT NOT NULL DEFAULT 'active'"},
	{"package_quarantine_metadata", "release_error", "TEXT NOT NULL DEFAULT ''"},
	{"package_quarantine_metadata", "snapshot_json", "TEXT NOT NULL DEFAULT ''"},
	{"package_quarantine_metadata", "snapshot_hash", "TEXT NOT NULL DEFAULT ''"},
	{"package_quarantine_metadata", "expected_generation_id", "TEXT NOT NULL DEFAULT ''"},
	{"package_quarantine_metadata", "expected_version_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_operation_steps", "result_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_operations", "snapshot_requirement_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "artifact_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "artifact_policy", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "policy_reason", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "current_version_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "current_generation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_preview_sessions", "preview_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_operations", "confirmation_claims_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "snapshot_requirement_hash", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "snapshot_requirement_json", "TEXT NOT NULL DEFAULT '{}'"},
	{"extension_package_rollback_points", "source_version_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "source_generation_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_package_rollback_points", "snapshot_id", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "pause_reason", "TEXT NOT NULL DEFAULT ''"},
	{"extension_workflow_executions", "pause_requested_at", "DATETIME"},
	{"extension_workflow_executions", "paused_at", "DATETIME"},
}

func ensureSchemaColumns(ctx context.Context, db dbExecutor) error {
	for _, a := range schemaColumnAdditions {
		tableExists, err := tableExists(ctx, db, a.table)
		if err != nil {
			return err
		}
		if !tableExists {
			continue
		}
		exists, err := columnExists(ctx, db, a.table, a.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", a.table, a.column, a.def)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if isDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("add column %s.%s: %w", a.table, a.column, err)
		}
	}
	return nil
}

func tableExists(ctx context.Context, db dbExecutor, table string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return name == table, nil
}

func columnExists(ctx context.Context, db dbExecutor, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func MigrationCount() int {
	return len(schemaMigrations)
}
