package migration

func DesktopPetEditingV3Migration() Migration {
	return Migration{
		Version: "202608020003",
		Name:    "editing_step9_draft_snapshot_and_job_enhancements",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_draft_snapshots (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  base_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  action_config_snapshot_json TEXT NOT NULL DEFAULT '{}',
  action_config_hash TEXT NOT NULL DEFAULT '',
  frames_json TEXT NOT NULL DEFAULT '[]',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  snapshot_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)

			s.CreateIndex("idx_deds_session", "desktop_pet_edit_draft_snapshots", []string{"session_id"}, false)
			s.CreateIndex("idx_deds_hash", "desktop_pet_edit_draft_snapshots", []string{"snapshot_hash"}, false)
			s.CreateIndex("uq_deds_session_version", "desktop_pet_edit_draft_snapshots", []string{"session_id", "session_version"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_job_input_snapshots (
  job_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_hash TEXT NOT NULL DEFAULT '',
  request_json TEXT NOT NULL DEFAULT '{}',
  request_hash TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  base_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  target_frame_id TEXT NOT NULL DEFAULT '',
  target_frame_content_hash TEXT NOT NULL DEFAULT '',
  generation_profile_id TEXT NOT NULL DEFAULT '',
  processing_profile_id TEXT NOT NULL DEFAULT '',
  quality_profile_id TEXT NOT NULL DEFAULT '',
  cost_estimate_json TEXT NOT NULL DEFAULT '{}',
  cost_estimate_hash TEXT NOT NULL DEFAULT '',
  cost_confirmation_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)

			s.CreateIndex("idx_dris_session", "desktop_pet_regeneration_job_input_snapshots", []string{"session_id"}, false)
			s.CreateIndex("idx_dris_draft", "desktop_pet_regeneration_job_input_snapshots", []string{"draft_snapshot_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_candidate_acceptance_operations (
  id TEXT PRIMARY KEY,
  candidate_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("uq_dcao_candidate_idem", "desktop_pet_candidate_acceptance_operations", []string{"candidate_id", "idempotency_key"}, true)
			s.CreateIndex("idx_dcao_session", "desktop_pet_candidate_acceptance_operations", []string{"session_id"}, false)

			s.AddColumn("desktop_pet_candidate_acceptance_operations", "updated_at", "TEXT NOT NULL DEFAULT ''")

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_editing_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'editing_job',
  aggregate_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("uq_deeo_event", "desktop_pet_editing_event_outbox", []string{"event_id"}, true)
			s.CreateIndex("idx_deeo_status", "desktop_pet_editing_event_outbox", []string{"status", "available_at"}, false)
			s.CreateIndex("idx_deeo_user", "desktop_pet_editing_event_outbox", []string{"user_id"}, false)

			s.AddColumn("desktop_pet_regeneration_jobs", "user_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "character_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "action_stream_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "draft_snapshot_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "draft_snapshot_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "stage", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "generation_attempt_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "generation_artifact_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "processing_attempt_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "execution_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "attempt_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_regeneration_jobs", "instance_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "cancel_requested_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "completed_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "supersedes_job_id", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_edit_sessions", "character_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_sessions", "action_stream_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_sessions", "draft_snapshot_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_sessions", "draft_snapshot_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_sessions", "closed_at", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_edit_candidates", "user_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "character_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "action_stream_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "candidate_version", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_edit_candidates", "draft_snapshot_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "draft_snapshot_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "parent_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "effective_verdict", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "activation_policy", "TEXT NOT NULL DEFAULT ''")

			s.CreateIndex("idx_drj_user", "desktop_pet_regeneration_jobs", []string{"user_id"}, false)
			s.CreateIndex("idx_drj_execution", "desktop_pet_regeneration_jobs", []string{"execution_id"}, false)
			s.CreateIndex("idx_drj_stage", "desktop_pet_regeneration_jobs", []string{"stage"}, false)
			s.CreateIndex("idx_des_character", "desktop_pet_edit_sessions", []string{"character_id"}, false)
			s.CreateIndex("idx_dec_user", "desktop_pet_edit_candidates", []string{"user_id"}, false)
			s.CreateIndex("idx_dec_candidate_rev", "desktop_pet_edit_candidates", []string{"candidate_revision_id"}, false)

			return nil
		},
	}
}
