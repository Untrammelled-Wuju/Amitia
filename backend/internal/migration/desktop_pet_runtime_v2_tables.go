// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeV2TablesMigration() Migration {
	return Migration{
		Version:           "202608020004",
		Name:              "add_desktop_pet_runtime_v2_command_tables",
		AcceptedChecksums: []string{"91c6056b4754c0f9575eaf07a260ee5c2c6e6dfead82e6c41c083763d1987c62"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  runtime_version TEXT NOT NULL DEFAULT '',
  runtime_contract_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '{}',
  capabilities_hash TEXT NOT NULL DEFAULT '',
  last_applied_desired_revision INTEGER NOT NULL DEFAULT 0,
  last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT '',
  connected_at TEXT NOT NULL DEFAULT '',
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
  disconnected_at TEXT NOT NULL DEFAULT '',
  superseded_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)
			s.AddColumn("desktop_pet_runtime_sessions", "runtime_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_runtime_sessions", "runtime_instance_id", "TEXT NOT NULL DEFAULT ''")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device ON desktop_pet_runtime_sessions(user_id, device_id)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device_runtime_status ON desktop_pet_runtime_sessions(user_id, device_id, runtime_id, status)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_commands_v2 (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  command_type TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  revision INTEGER NOT NULL DEFAULT 0,
  device_sequence INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
  runtime_correlation_id TEXT NOT NULL DEFAULT '',
  runtime_playback_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  hash_code INTEGER NOT NULL DEFAULT 0,
  attempt INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT '',
  dispatch_at TEXT NOT NULL DEFAULT '',
  transport_dispatched_at TEXT NOT NULL DEFAULT '',
  runtime_received_at TEXT NOT NULL DEFAULT '',
  runtime_accepted_at TEXT NOT NULL DEFAULT '',
  renderer_accepted_at TEXT NOT NULL DEFAULT '',
  playback_started_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT '',
  superseded_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
  UNIQUE(user_id, device_id, idempotency_key)
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtcv2_user_device_type ON desktop_pet_runtime_commands_v2(user_id, device_id, command_type)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtcv2_status ON desktop_pet_runtime_commands_v2(status, inserted_at)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtcv2_device_seq ON desktop_pet_runtime_commands_v2(user_id, device_id, device_sequence)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_event_records (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  command_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  occurred_at TEXT NOT NULL DEFAULT '',
  delivered INTEGER NOT NULL DEFAULT 0,
  delivered_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  UNIQUE(runtime_session_id, sequence)
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rter_session_seq ON desktop_pet_runtime_event_records(runtime_session_id, sequence)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rter_session_delivered ON desktop_pet_runtime_event_records(runtime_session_id, delivered, sequence)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_actual_states_v2 (
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0,
  applied_desired_revision INTEGER NOT NULL DEFAULT 0,
  applied_desired_hash TEXT NOT NULL DEFAULT '',
  applied_settings_revision INTEGER NOT NULL DEFAULT 0,
  installation_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  instance_status TEXT NOT NULL DEFAULT '',
  window_status TEXT NOT NULL DEFAULT '',
  renderer_status TEXT NOT NULL DEFAULT '',
  playback_status TEXT NOT NULL DEFAULT '',
  visible INTEGER NOT NULL DEFAULT 0,
  stable_action_key TEXT NOT NULL DEFAULT '',
  current_action_key TEXT NOT NULL DEFAULT '',
  playback_instance_id TEXT NOT NULL DEFAULT '',
  current_command_id TEXT NOT NULL DEFAULT '',
  actual_state_hash TEXT NOT NULL DEFAULT '',
  health_status TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
  updated_at TEXT DEFAULT '',
  PRIMARY KEY(user_id, device_id, runtime_id)
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtasv2_user_device ON desktop_pet_runtime_actual_states_v2(user_id, device_id)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_attempts (
  attempt_id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  dispatched_at TEXT NOT NULL DEFAULT '',
  runtime_received_at TEXT NOT NULL DEFAULT '',
  runtime_accepted_at TEXT NOT NULL DEFAULT '',
  renderer_accepted_at TEXT NOT NULL DEFAULT '',
  playback_started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtca_command ON desktop_pet_runtime_command_attempts(command_id)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_results (
  id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  attempt_id TEXT NOT NULL DEFAULT '',
  result_type TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}',
  result_hash TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  event_sequence INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT '',
  UNIQUE(command_id, runtime_id)
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtcr_command ON desktop_pet_runtime_command_results(command_id)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_rtcr_runtime ON desktop_pet_runtime_command_results(runtime_id)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_device_command_sequences (
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  last_reserved_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT '',
  PRIMARY KEY(user_id, device_id)
)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_domain_event_outbox (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
  claim_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT '',
  published_at TEXT NOT NULL DEFAULT ''
)`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_dteo_status ON desktop_pet_runtime_domain_event_outbox(status, inserted_at)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_dedup (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  nak_count INTEGER NOT NULL DEFAULT 0,
  last_nak_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)
			s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_rtcdd_user_device_idem ON desktop_pet_runtime_command_dedup(user_id, device_id, idempotency_key)")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_reconcile_leases (
  reconciler_id TEXT PRIMARY KEY,
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)

			return nil
		},
	}
}
