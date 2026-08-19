// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetQualityBridgeMigration() Migration {
	return Migration{
		Version:           "202607310015",
		Name:              "add_desktop_pet_quality_bridge_tables",
		AcceptedChecksums: []string{"c63a39b7fd6da59ce5d1aba589c1ee708f91993bb531e20110c1a711b5ac2c4a"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_evaluation_bindings (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL,
  profile_id TEXT NOT NULL DEFAULT '',
  active_evaluation_id TEXT NOT NULL,
  binding_revision INTEGER NOT NULL DEFAULT 1,
  bound_at TEXT NOT NULL DEFAULT '',
created_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)
	if err := s.CreateIndex("uq_dpaqeb_revision_profile", "desktop_pet_active_quality_evaluation_bindings", []string{"action_revision_id", "profile_id"}, true); err != nil {
		return err
	}

	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_commit_journals (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  commit_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  steps_completed TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT '',
  completed_at TEXT DEFAULT ''
)`)
	if err := s.CreateIndex("idx_dpqcj_eval", "desktop_pet_quality_commit_journals", []string{"evaluation_id"}, false); err != nil {
		return err
	}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_review_decisions (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  action_revision_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  reviewer TEXT NOT NULL DEFAULT '',
  reviewed_at TEXT NOT NULL DEFAULT '',
created_at TEXT DEFAULT ''
)`)
	if err := s.CreateIndex("uq_dpqrd_eval", "desktop_pet_quality_review_decisions", []string{"evaluation_id"}, true); err != nil {
		return err
	}
	s.CreateIndex("idx_dpqrd_revision", "desktop_pet_quality_review_decisions", []string{"action_revision_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_measurement_cache (
  id TEXT PRIMARY KEY,
  frame_artifact_id TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  measurement_version TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  has_alpha_channel INTEGER NOT NULL DEFAULT 0,
  alpha_coverage REAL NOT NULL DEFAULT 0,
  fully_transparent_ratio REAL NOT NULL DEFAULT 0,
  semi_transparent_ratio REAL NOT NULL DEFAULT 0,
  opaque_ratio REAL NOT NULL DEFAULT 0,
  decodable INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT NOT NULL DEFAULT '',
  pixel_hash TEXT NOT NULL DEFAULT '',
  measurements_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT DEFAULT ''
)`)
	if err := s.CreateIndex("uq_dpqmc_artifact_hash_ver", "desktop_pet_quality_measurement_cache", []string{"frame_artifact_id", "content_hash", "measurement_version"}, true); err != nil {
		return err
	}

	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_outbox_events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT DEFAULT '',
  published_at TEXT DEFAULT ''
)`)
	if err := s.CreateIndex("idx_dpqoe_status", "desktop_pet_quality_outbox_events", []string{"status"}, false); err != nil {
		return err
	}

			s.AddColumn("desktop_pet_quality_evaluations", "user_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "character_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "action_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "processing_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "profile_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "profile_version", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "rule_set_version", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "ruleset_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "measurement_version", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "lease_owner", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "heartbeat_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_evaluations", "attempt_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_quality_evaluations", "idempotency_key", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_quality_gate_results", "active_revision_set_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_gate_results", "evaluation_set_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_gate_results", "profile_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_quality_gate_results", "invalidated_at", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_action_revisions", "quality_profile_id", "TEXT NOT NULL DEFAULT ''")

			return nil
		},
	}
}
