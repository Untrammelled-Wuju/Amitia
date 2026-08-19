// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetQualitySystemMigration() Migration {
	return Migration{
		Version: "202607310006",
		Name:    "add_desktop_pet_quality_system_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_evaluations (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL,
  processing_action_id TEXT NOT NULL,
  action_revision_id TEXT NOT NULL,
  measurement_set_id TEXT NOT NULL,
  action_key TEXT NOT NULL,
  execution_status TEXT NOT NULL DEFAULT 'pending',
  verdict TEXT DEFAULT '',
  overall_score REAL,
  overall_confidence REAL NOT NULL DEFAULT 0,
  profile_snapshot_json TEXT NOT NULL,
  profile_hash TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  quality_mode TEXT DEFAULT 'balanced',
  report_path TEXT DEFAULT '',
  report_hash TEXT DEFAULT '',
  supersedes_evaluation_id TEXT DEFAULT '',
  execution_id TEXT DEFAULT '',
  worker_id TEXT DEFAULT '',
  lease_expires_at TEXT DEFAULT '',
  error_code TEXT DEFAULT '',
  error_message TEXT DEFAULT '',
  is_active INTEGER NOT NULL DEFAULT 0,
  started_at TEXT DEFAULT '',
  completed_at TEXT DEFAULT '',
  created_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)

			s.CreateIndex("uq_dpqe_input", "desktop_pet_quality_evaluations", []string{"action_revision_id", "profile_hash", "engine_version"}, true)
			s.CreateIndex("idx_dpqe_task", "desktop_pet_quality_evaluations", []string{"processing_task_id"}, false)
			s.CreateIndex("idx_dpqe_action", "desktop_pet_quality_evaluations", []string{"processing_action_id"}, false)
			s.CreateIndex("idx_dpqe_status", "desktop_pet_quality_evaluations", []string{"execution_status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_findings (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  rule_code TEXT NOT NULL,
  rule_version INTEGER NOT NULL,
  dimension_key TEXT NOT NULL,
  severity TEXT NOT NULL,
  hard_gate INTEGER NOT NULL DEFAULT 0,
  frame_indexes_json TEXT NOT NULL DEFAULT '[]',
  frame_pairs_json TEXT NOT NULL DEFAULT '[]',
  regions_json TEXT NOT NULL DEFAULT '[]',
  metric_name TEXT DEFAULT '',
  observed_value REAL,
  threshold_value REAL,
  comparison TEXT DEFAULT '',
  confidence REAL NOT NULL DEFAULT 0,
  message_key TEXT NOT NULL,
  message TEXT NOT NULL,
  suggested_action TEXT DEFAULT '',
  evidence_ref TEXT DEFAULT '',
  sort_key TEXT NOT NULL,
  created_at TEXT DEFAULT ''
)`)

			s.CreateIndex("idx_dpqf_eval", "desktop_pet_quality_findings", []string{"evaluation_id"}, false)
			s.CreateIndex("idx_dpqf_rule", "desktop_pet_quality_findings", []string{"rule_code"}, false)
			s.CreateIndex("idx_dpqf_severity", "desktop_pet_quality_findings", []string{"severity"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_dimension_scores (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  dimension_key TEXT NOT NULL,
  applicability TEXT NOT NULL,
  score REAL,
  confidence REAL NOT NULL DEFAULT 0,
  weight REAL NOT NULL DEFAULT 0,
  details_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT DEFAULT '',
  UNIQUE(evaluation_id, dimension_key)
)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_results (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL,
  gate_status TEXT NOT NULL,
  required_action_count INTEGER NOT NULL DEFAULT 0,
  accepted_action_count INTEGER NOT NULL DEFAULT 0,
  warning_action_count INTEGER NOT NULL DEFAULT 0,
  review_action_count INTEGER NOT NULL DEFAULT 0,
  rejected_action_count INTEGER NOT NULL DEFAULT 0,
  failed_evaluation_count INTEGER NOT NULL DEFAULT 0,
  snapshot_json TEXT NOT NULL,
  snapshot_hash TEXT NOT NULL,
  created_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)

			s.CreateIndex("uq_dpqg_task_snapshot", "desktop_pet_quality_gate_results", []string{"processing_task_id", "snapshot_hash"}, true)

			return nil
		},
	}
}
