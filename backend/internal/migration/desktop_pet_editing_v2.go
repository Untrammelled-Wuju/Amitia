package migration

func DesktopPetEditingV2Migration() Migration {
	return Migration{
		Version: "202607310014",
		Name:    "add_desktop_pet_editing_v2_regeneration_candidate",
		Up: func(s *Step) error {
			s.AddColumn("desktop_pet_edit_sessions", "base_action_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_sessions", "base_binding_revision", "INTEGER NOT NULL DEFAULT 0")

			s.AddColumn("desktop_pet_regeneration_jobs", "mode", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "active_attempt_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "provider_receipt_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "request_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "artifact_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "processing_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "candidate_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "quality_evaluation_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "lease_owner", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "lease_expires_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "heartbeat_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "base_action_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "base_content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "base_binding_revision", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_regeneration_jobs", "reject_reason", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "rejected_by", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_regeneration_jobs", "rejected_at", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_edit_candidates", "source_type", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "parent_action_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "base_binding_revision", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_edit_candidates", "quality_status", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "quality_evaluation_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "content_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "frame_set_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "action_config_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "accepted_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "rejected_at", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_edit_candidates", "reject_reason", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_action_generation_attempts", "origin", "TEXT NOT NULL DEFAULT 'generation_task'")
			s.AddColumn("desktop_pet_action_generation_attempts", "regeneration_job_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_action_generation_attempts", "edit_session_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_action_generation_attempts", "base_action_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_action_generation_attempts", "target_frame_index", "INTEGER NOT NULL DEFAULT -1")

			s.CreateIndex("idx_drj_lease", "desktop_pet_regeneration_jobs", []string{"lease_owner", "lease_expires_at"}, false)
			s.CreateIndex("idx_drj_candidate", "desktop_pet_regeneration_jobs", []string{"candidate_revision_id"}, false)
			s.CreateIndex("idx_dec_revision", "desktop_pet_edit_candidates", []string{"candidate_revision_id"}, false)
			s.CreateIndex("idx_dec_status_quality", "desktop_pet_edit_candidates", []string{"status", "quality_status"}, false)
			s.CreateIndex("idx_daga_origin_regen", "desktop_pet_action_generation_attempts", []string{"origin", "regeneration_job_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_journals (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  attempt_id TEXT NOT NULL DEFAULT '',
  provider_receipt_id TEXT NOT NULL DEFAULT '',
  artifact_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
quality_evaluation_id TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_drj_job", "desktop_pet_regeneration_journals", []string{"job_id"}, false)
			s.CreateIndex("idx_drj_state", "desktop_pet_regeneration_journals", []string{"state"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_candidate_revision_metadata (
  id TEXT PRIMARY KEY,
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  parent_action_revision_id TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  regeneration_job_id TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
action_config_hash TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'candidate_committing',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("uq_dcrm_candidate", "desktop_pet_candidate_revision_metadata", []string{"candidate_revision_id"}, true)
			s.CreateIndex("idx_dcrm_job", "desktop_pet_candidate_revision_metadata", []string{"regeneration_job_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_audit_logs (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  edit_session_id TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  previous_active_revision_id TEXT NOT NULL DEFAULT '',
  new_active_revision_id TEXT NOT NULL DEFAULT '',
reason TEXT NOT NULL DEFAULT '',
occurred_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_deal_session", "desktop_pet_edit_audit_logs", []string{"edit_session_id"}, false)
			s.CreateIndex("idx_deal_event", "desktop_pet_edit_audit_logs", []string{"event_type"}, false)

			return nil
		},
	}
}
