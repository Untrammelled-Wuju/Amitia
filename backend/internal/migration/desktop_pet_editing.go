package migration

func DesktopPetEditingMigration() Migration {
	return Migration{
		Version:           "202607300032",
		Name:              "add_desktop_pet_editing_tables",
		AcceptedChecksums: []string{"2ea95a62ad598ddef93601a628175168ae13660aa61c47c3223ebe5e45bc4eda"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revisions (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  generation_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  parent_revision_id TEXT NOT NULL DEFAULT '',
  root_revision_id TEXT NOT NULL DEFAULT '',
  revision_number INTEGER NOT NULL DEFAULT 1,
  revision_type TEXT NOT NULL DEFAULT 'processed',
  status TEXT NOT NULL DEFAULT 'building',
  manifest_path TEXT NOT NULL DEFAULT '',
  manifest_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  default_fps INTEGER NOT NULL DEFAULT 0,
  loop_type TEXT NOT NULL DEFAULT '',
  return_action TEXT NOT NULL DEFAULT '',
  interruptible INTEGER NOT NULL DEFAULT 1,
  priority_override INTEGER,
  cooldown_ms_override INTEGER,
  quality_evaluation_id TEXT NOT NULL DEFAULT '',
  quality_verdict TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT NOT NULL DEFAULT '',
  created_from_session_id TEXT NOT NULL DEFAULT '',
  change_summary TEXT NOT NULL DEFAULT '',
  source_summary_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  ready_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dpar_task", "desktop_pet_action_revisions", []string{"processing_task_id"}, false)
			s.CreateIndex("idx_dpar_action", "desktop_pet_action_revisions", []string{"processing_action_id"}, false)
			s.CreateIndex("idx_dpar_parent", "desktop_pet_action_revisions", []string{"parent_revision_id"}, false)
			s.CreateIndex("uq_dpar_task_action_rev", "desktop_pet_action_revisions", []string{"processing_task_id", "action_key", "revision_number"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_active_revisions (
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  revision_id TEXT NOT NULL DEFAULT '',
  binding_version INTEGER NOT NULL DEFAULT 0,
  activated_by TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(processing_task_id, action_key)
)`)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_frame_assets (
  id TEXT PRIMARY KEY,
  content_hash TEXT NOT NULL DEFAULT '',
  storage_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  byte_size INTEGER NOT NULL DEFAULT 0,
  alpha_mode TEXT NOT NULL DEFAULT '',
  color_space TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  source_ref_id TEXT NOT NULL DEFAULT '',
  original_hash TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'staging',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("uq_dfa_hash", "desktop_pet_frame_assets", []string{"content_hash", "mime_type"}, true)
			s.CreateIndex("idx_dfa_source", "desktop_pet_frame_assets", []string{"source_type", "source_ref_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_frames (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_id TEXT NOT NULL DEFAULT '',
  asset_id TEXT NOT NULL DEFAULT '',
  logical_index INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 100,
  source_frame_id TEXT NOT NULL DEFAULT '',
  source_revision_id TEXT NOT NULL DEFAULT '',
  source_attempt_id TEXT NOT NULL DEFAULT '',
  anchor_x REAL NOT NULL DEFAULT 0.5,
  anchor_y REAL NOT NULL DEFAULT 0.9,
  anchor_space TEXT NOT NULL DEFAULT 'normalized_canvas',
  offset_x REAL NOT NULL DEFAULT 0,
  offset_y REAL NOT NULL DEFAULT 0,
  mask_asset_id TEXT NOT NULL DEFAULT '',
  transform_json TEXT NOT NULL DEFAULT '{}',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  copied_from_frame_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dparf_revision", "desktop_pet_action_revision_frames", []string{"revision_id"}, false)
			s.CreateIndex("uq_dparf_rev_index", "desktop_pet_action_revision_frames", []string{"revision_id", "logical_index"}, true)
			s.CreateIndex("uq_dparf_rev_frame_id", "desktop_pet_action_revision_frames", []string{"revision_id", "frame_id"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  cursor INTEGER NOT NULL DEFAULT 0,
  last_operation_seq INTEGER NOT NULL DEFAULT 0,
  checkpoint_id TEXT NOT NULL DEFAULT '',
  client_instance_id TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  committed_revision_id TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_des_user", "desktop_pet_edit_sessions", []string{"user_id"}, false)
			s.CreateIndex("idx_des_task_action", "desktop_pet_edit_sessions", []string{"processing_task_id", "action_key"}, false)
			s.CreateIndex("idx_des_status", "desktop_pet_edit_sessions", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_operations (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  operation_type TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  inverse_json TEXT NOT NULL DEFAULT '{}',
  idempotency_key TEXT NOT NULL DEFAULT '',
  base_version INTEGER NOT NULL DEFAULT 0,
  result_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'applied',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_deo_session", "desktop_pet_edit_operations", []string{"session_id"}, false)
			s.CreateIndex("uq_deo_session_seq", "desktop_pet_edit_operations", []string{"session_id", "sequence"}, true)
			s.CreateIndex("uq_deo_idempotency", "desktop_pet_edit_operations", []string{"session_id", "idempotency_key"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_checkpoints (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  manifest_json TEXT NOT NULL DEFAULT '{}',
  manifest_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dec_session", "desktop_pet_edit_checkpoints", []string{"session_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_jobs (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  target_frame_id TEXT NOT NULL DEFAULT '',
  job_type TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  idempotency_key TEXT NOT NULL DEFAULT '',
  provider_attempt_id TEXT NOT NULL DEFAULT '',
  request_snapshot_json TEXT NOT NULL DEFAULT '{}',
  cost_estimate_json TEXT NOT NULL DEFAULT '{}',
  cost_actual_json TEXT NOT NULL DEFAULT '{}',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_drj_session", "desktop_pet_regeneration_jobs", []string{"session_id"}, false)
			s.CreateIndex("idx_drj_status", "desktop_pet_regeneration_jobs", []string{"status"}, false)
			s.CreateIndex("uq_drj_idempotency", "desktop_pet_regeneration_jobs", []string{"session_id", "idempotency_key"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_candidates (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  target_frame_id TEXT NOT NULL DEFAULT '',
  candidate_type TEXT NOT NULL DEFAULT '',
  asset_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  decided_by TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dec_session_job", "desktop_pet_edit_candidates", []string{"session_id", "job_id"}, false)
			s.CreateIndex("idx_dec_status", "desktop_pet_edit_candidates", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_mask_patches (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  frame_id TEXT NOT NULL DEFAULT '',
  source_asset_hash TEXT NOT NULL DEFAULT '',
  result_asset_id TEXT NOT NULL DEFAULT '',
  patch_type TEXT NOT NULL DEFAULT '',
  brush_data_path TEXT NOT NULL DEFAULT '',
  brush_size INTEGER NOT NULL DEFAULT 0,
  brush_hardness REAL NOT NULL DEFAULT 0,
  brush_opacity REAL NOT NULL DEFAULT 1,
  coordinate_space TEXT NOT NULL DEFAULT 'normalized_canvas',
  canvas_width INTEGER NOT NULL DEFAULT 0,
  canvas_height INTEGER NOT NULL DEFAULT 0,
  algorithm_version TEXT NOT NULL DEFAULT '',
  operation_seq INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dmp_session_frame", "desktop_pet_mask_patches", []string{"session_id", "frame_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_publish_journal (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  tmp_dir_path TEXT NOT NULL DEFAULT '',
  final_dir_path TEXT NOT NULL DEFAULT '',
  manifest_path TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dpj_revision", "desktop_pet_publish_journal", []string{"revision_id"}, false)
			s.CreateIndex("idx_dpj_status", "desktop_pet_publish_journal", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_edit_idempotency (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("uq_dei_user_session_key", "desktop_pet_edit_idempotency", []string{"user_id", "session_id", "idempotency_key"}, true)

			return nil
		},
	}
}
