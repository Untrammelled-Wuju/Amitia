// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetQualityV2Migration() Migration {
	return Migration{
		Version:           "202607310016",
		Name:              "add_desktop_pet_quality_v2_tables",
		AcceptedChecksums: []string{"c63a39b7fd6da59ce5d1aba589c1ee708f91993bb531e20110c1a711b5ac2c4a", "57eeb72aaa9dbbafb8eff2cf883475e76be10f17ad99c0817a59bf49da6cdfef"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_evaluation_request_inbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  profile_id TEXT NOT NULL DEFAULT '',
  profile_version TEXT NOT NULL DEFAULT '',
  rule_set_version TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'received',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dpeqri_event", "desktop_pet_quality_evaluation_request_inbox", []string{"event_id"}, true)
			s.CreateIndex("uq_dpeqri_idem", "desktop_pet_quality_evaluation_request_inbox", []string{"idempotency_key"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_input_snapshots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  processing_revision_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  action_config_hash TEXT NOT NULL DEFAULT '',
  action_spec_hash TEXT NOT NULL DEFAULT '',
  playback_mode TEXT NOT NULL DEFAULT '',
  fps INTEGER NOT NULL DEFAULT 0,
  expected_frame_count INTEGER NOT NULL DEFAULT 0,
  frame_inputs_json TEXT NOT NULL DEFAULT '[]',
  snapshot_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dpqis_snap", "desktop_pet_quality_input_snapshots", []string{"action_revision_id", "snapshot_hash"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_measurement_sets (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  measurement_version TEXT NOT NULL DEFAULT '',
  measurement_profile_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  canvas_width INTEGER NOT NULL DEFAULT 0,
  canvas_height INTEGER NOT NULL DEFAULT 0,
  measurement_set_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'building',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dpqms_set", "desktop_pet_quality_measurement_sets", []string{"action_revision_id", "measurement_set_hash"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_frame_measurements (
  id TEXT PRIMARY KEY,
  measurement_set_id TEXT NOT NULL DEFAULT '',
  frame_artifact_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  file_hash TEXT NOT NULL DEFAULT '',
  pixel_hash TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT NOT NULL DEFAULT '',
  file_bytes INTEGER NOT NULL DEFAULT 0,
  has_alpha_channel INTEGER NOT NULL DEFAULT 0,
  alpha_coverage REAL NOT NULL DEFAULT 0,
  fully_transparent_ratio REAL NOT NULL DEFAULT 0,
  semi_transparent_ratio REAL NOT NULL DEFAULT 0,
  opaque_ratio REAL NOT NULL DEFAULT 0,
  decodable INTEGER NOT NULL DEFAULT 0,
  subject_box_json TEXT NOT NULL DEFAULT '{}',
  transform_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpqfm_set", "desktop_pet_quality_frame_measurements", []string{"measurement_set_id"}, false)
			s.CreateIndex("idx_dpqfm_frame", "desktop_pet_quality_frame_measurements", []string{"frame_artifact_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_sequence_measurements (
  id TEXT PRIMARY KEY,
  measurement_set_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  subject_area_ratio REAL NOT NULL DEFAULT 0,
  connected_component_count INTEGER NOT NULL DEFAULT 0,
  largest_component_ratio REAL NOT NULL DEFAULT 0,
  border_foreground_coverage REAL NOT NULL DEFAULT 0,
  edge_contact_json TEXT NOT NULL DEFAULT '[]',
  centroid_x REAL NOT NULL DEFAULT 0,
  centroid_y REAL NOT NULL DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpqsm_set", "desktop_pet_quality_sequence_measurements", []string{"measurement_set_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_report_artifacts (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL DEFAULT '',
  storage_key TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  byte_size INTEGER NOT NULL DEFAULT 0,
  schema_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'staging',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dpqra_eval", "desktop_pet_quality_report_artifacts", []string{"evaluation_id"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_binding_history (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL DEFAULT '',
  profile_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  previous_evaluation_id TEXT NOT NULL DEFAULT '',
  new_evaluation_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  actor TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_dpaqbh_rev", "desktop_pet_active_quality_binding_history", []string{"action_revision_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_snapshots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  evaluation_set_hash TEXT NOT NULL DEFAULT '',
  gate_profile_id TEXT NOT NULL DEFAULT '',
  gate_profile_version TEXT NOT NULL DEFAULT '',
  rule_set_version TEXT NOT NULL DEFAULT '',
  rule_set_content_hash TEXT NOT NULL DEFAULT '',
  gate_status TEXT NOT NULL DEFAULT '',
  required_action_keys_json TEXT NOT NULL DEFAULT '[]',
  included_action_keys_json TEXT NOT NULL DEFAULT '[]',
  excluded_action_keys_json TEXT NOT NULL DEFAULT '[]',
  action_verdicts_json TEXT NOT NULL DEFAULT '[]',
  required_action_count INTEGER NOT NULL DEFAULT 0,
  accepted_action_count INTEGER NOT NULL DEFAULT 0,
  warning_action_count INTEGER NOT NULL DEFAULT 0,
  review_action_count INTEGER NOT NULL DEFAULT 0,
  rejected_action_count INTEGER NOT NULL DEFAULT 0,
  failed_evaluation_count INTEGER NOT NULL DEFAULT 0,
  snapshot_hash TEXT NOT NULL DEFAULT '',
  gate_hash TEXT NOT NULL DEFAULT '',
  invalidated_at TEXT NOT NULL DEFAULT '',
  invalidation_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpqgs_task", "desktop_pet_quality_gate_snapshots", []string{"processing_task_id"}, false)
			s.CreateIndex("idx_dpqgs_rsh", "desktop_pet_quality_gate_snapshots", []string{"active_revision_set_hash"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_gate_bindings (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  gate_profile_hash TEXT NOT NULL DEFAULT '',
  active_gate_id TEXT NOT NULL DEFAULT '',
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  evaluation_set_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dpaqgbind", "desktop_pet_active_quality_gate_bindings", []string{"processing_task_id", "gate_profile_hash"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_rebuild_requests (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  source_event_type TEXT NOT NULL DEFAULT '',
  source_event_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_dpqgrr_task", "desktop_pet_quality_gate_rebuild_requests", []string{"processing_task_id"}, false)

			s.AddColumn("desktop_pet_quality_evaluations", "action_stream_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "frame_set_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "binding_revision", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_quality_evaluations", "input_snapshot_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "measurement_set_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "report_artifact_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "result_hash", "TEXT NOT NULL DEFAULT ''")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_outbox_events_v2 (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("uq_dpqoev2_event", "desktop_pet_quality_outbox_events_v2", []string{"event_id"}, true)
			s.CreateIndex("uq_dpqoev2_agg", "desktop_pet_quality_outbox_events_v2", []string{"aggregate_id", "aggregate_sequence", "event_type"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_commit_journals_v2 (
  id TEXT PRIMARY KEY,
  commit_hash TEXT NOT NULL DEFAULT '',
  evaluation_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  steps_json TEXT NOT NULL DEFAULT '',
  report_staging_key TEXT NOT NULL DEFAULT '',
  report_final_key TEXT NOT NULL DEFAULT '',
  report_hash TEXT NOT NULL DEFAULT '',
  result_hash TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_dpqcjv2_eval", "desktop_pet_quality_commit_journals_v2", []string{"evaluation_id"}, false)
			s.CreateIndex("uq_dpqcjv2_hash", "desktop_pet_quality_commit_journals_v2", []string{"commit_hash"}, true)

			return nil
		},
	}
}