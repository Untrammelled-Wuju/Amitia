package migration

func DesktopPetProcessingRevisionsMigration() Migration {
	return Migration{
		Version: "202607300031",
		Name:    "add_desktop_pet_processing_revisions_and_artifacts",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_revisions (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  revision_number INTEGER NOT NULL DEFAULT 1,
  source_attempt_id TEXT NOT NULL DEFAULT '',
  source_candidate_index INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'preparing',
  config_snapshot TEXT NOT NULL DEFAULT '{}',
  config_hash TEXT NOT NULL DEFAULT '',
  pipeline_version TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  root_relative_path TEXT NOT NULL DEFAULT '',
  revision_hash TEXT NOT NULL DEFAULT '',
  active INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  published_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dppr_task", "desktop_pet_processing_revisions", []string{"processing_task_id"}, false)
			s.CreateIndex("idx_dppr_action", "desktop_pet_processing_revisions", []string{"processing_action_id"}, false)
			s.CreateIndex("uq_dppr_action_revision", "desktop_pet_processing_revisions", []string{"processing_action_id", "revision_number"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_artifacts (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER,
  artifact_kind TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT '',
  relative_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  byte_size INTEGER NOT NULL DEFAULT 0,
  content_hash TEXT NOT NULL DEFAULT '',
  source_artifact_id TEXT NOT NULL DEFAULT '',
  source_cell_index INTEGER,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dppa_revision", "desktop_pet_processing_artifacts", []string{"revision_id"}, false)
			s.CreateIndex("uq_dppa_rev_kind_frame", "desktop_pet_processing_artifacts", []string{"revision_id", "artifact_kind", "frame_index"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_transforms (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  sequence_number INTEGER NOT NULL DEFAULT 0,
  from_space TEXT NOT NULL DEFAULT '',
  to_space TEXT NOT NULL DEFAULT '',
  transform_type TEXT NOT NULL DEFAULT '',
  matrix_json TEXT NOT NULL DEFAULT '[]',
  parameters_json TEXT NOT NULL DEFAULT '{}',
  algorithm_version TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dppt_revision", "desktop_pet_processing_transforms", []string{"revision_id"}, false)
			s.CreateIndex("uq_dppt_rev_frame_seq", "desktop_pet_processing_transforms", []string{"revision_id", "frame_index", "sequence_number"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_frame_measurements (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  measurement_schema_version INTEGER NOT NULL DEFAULT 1,
  subject_box_json TEXT NOT NULL DEFAULT '{}',
  source_anchor_json TEXT NOT NULL DEFAULT '{}',
  target_anchor_json TEXT NOT NULL DEFAULT '{}',
  alpha_coverage REAL NOT NULL DEFAULT 0,
  component_count INTEGER NOT NULL DEFAULT 0,
  edge_contact_json TEXT NOT NULL DEFAULT '{}',
  clipping_json TEXT NOT NULL DEFAULT '{}',
  trajectory_json TEXT NOT NULL DEFAULT '{}',
  measurement_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
)`)

			s.CreateIndex("idx_dpfm_revision", "desktop_pet_frame_measurements", []string{"revision_id"}, false)
			s.CreateIndex("uq_dpfm_rev_frame", "desktop_pet_frame_measurements", []string{"revision_id", "frame_index"}, true)

			s.AddColumn("desktop_pet_processing_tasks", "config_snapshot", "TEXT NOT NULL DEFAULT '{}'")
			s.AddColumn("desktop_pet_processing_tasks", "config_hash", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processing_tasks", "pipeline_version", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processing_tasks", "active_revision_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_processing_tasks", "publish_state", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("desktop_pet_processing_actions", "active_revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processing_actions", "next_revision_number", "INTEGER NOT NULL DEFAULT 1")
			s.AddColumn("desktop_pet_processing_actions", "source_attempt_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processing_actions", "source_candidate_index", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("desktop_pet_processing_actions", "processing_profile_snapshot", "TEXT NOT NULL DEFAULT '{}'")

			s.AddColumn("desktop_pet_processed_frames", "revision_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processed_frames", "mask_path", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processed_frames", "transform_chain_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processed_frames", "measurement_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processed_frames", "source_artifact_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("desktop_pet_processed_frames", "source_cell_index", "INTEGER")

			return nil
		},
	}
}
