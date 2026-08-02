package migration

func DesktopPetActionRevisionBridgeV2Migration() Migration {
	return Migration{
		Version: "202607310019",
		Name:    "add_desktop_pet_action_revision_bridge_v2",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_action_streams", "root_processing_task_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_streams", "stream_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute("DROP INDEX IF EXISTS uq_das_user_char_action")
			s.CreateIndex("uq_das_stream_key", "desktop_pet_action_streams", []string{"stream_key"}, true)

			if err := s.AddColumn("desktop_pet_action_revisions", "action_stream_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "source_processing_task_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "source_processing_action_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "source_processing_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "parent_action_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "root_action_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "action_config_snapshot_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "action_spec_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "revision_snapshot_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "revision_snapshot_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute("DROP INDEX IF EXISTS uq_dpar_source_type")
			s.CreateIndex("uq_dpar_source_type", "desktop_pet_action_revisions", []string{"source_processing_revision_id", "source_type"}, true)
			s.CreateIndex("idx_dpar_stream_rev", "desktop_pet_action_revisions", []string{"action_stream_id", "revision_number"}, false)

			if err := s.AddColumn("desktop_pet_frame_assets", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_frame_assets", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_frame_assets", "storage_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_frame_assets", "source_processing_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_frame_assets", "source_processing_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.CreateIndex("uq_dfa_source_artifact", "desktop_pet_frame_assets", []string{"source_type", "source_processing_artifact_id"}, true)

			if err := s.AddColumn("desktop_pet_action_revision_frames", "source_processing_frame_artifact_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revision_frames", "source_processing_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revision_frames", "source_processing_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revision_frames", "transform_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revision_frames", "measurement_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_active_action_revision_bindings", "action_stream_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute("DROP INDEX IF EXISTS uq_daarb_user_char_action")
			s.CreateIndex("uq_daarb_action_stream_id", "desktop_pet_active_action_revision_bindings", []string{"action_stream_id"}, true)

			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "event_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "action_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "payload_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "payload_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "lease_owner", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "lease_expires_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_revision_bridge_journals", "processed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute("DROP INDEX IF EXISTS idx_drbj_proc_rev")
			s.CreateIndex("uq_drbj_proc_rev", "desktop_pet_revision_bridge_journals", []string{"processing_revision_id"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_binding_history (
  id TEXT PRIMARY KEY,
  action_stream_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  previous_revision_id TEXT NOT NULL DEFAULT '',
  new_revision_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  actor TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
  correlation_id TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("uq_darbh_stream_rev", "desktop_pet_action_revision_binding_history", []string{"action_stream_id", "binding_revision"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_bridge_inbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'received',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT (datetime('now')),
  processed_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("uq_darbi_event_id", "desktop_pet_action_revision_bridge_inbox", []string{"event_id"}, true)
			s.CreateIndex("uq_darbi_proc_rev", "desktop_pet_action_revision_bridge_inbox", []string{"processing_revision_id"}, true)
			s.CreateIndex("idx_darbi_status", "desktop_pet_action_revision_bridge_inbox", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'action_revision',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  previous_revision_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("uq_dareo_event_id", "desktop_pet_action_revision_event_outbox", []string{"event_id"}, true)
			s.CreateIndex("uq_dareo_agg_seq_type", "desktop_pet_action_revision_event_outbox", []string{"aggregate_id", "aggregate_sequence", "event_type"}, true)
			s.CreateIndex("idx_dareo_status", "desktop_pet_action_revision_event_outbox", []string{"status"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_revision_mappings (
  id TEXT PRIMARY KEY,
  legacy_revision_id TEXT NOT NULL DEFAULT '',
  new_action_revision_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  legacy_revision_number INTEGER NOT NULL DEFAULT 0,
  migrated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dlrm_legacy_rev", "desktop_pet_legacy_revision_mappings", []string{"legacy_revision_id"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_legacy_binding_mappings (
  id TEXT PRIMARY KEY,
  legacy_processing_task_id TEXT NOT NULL DEFAULT '',
  legacy_action_key TEXT NOT NULL DEFAULT '',
  legacy_revision_id TEXT NOT NULL DEFAULT '',
  new_binding_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  migrated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_dlbm_legacy", "desktop_pet_legacy_binding_mappings", []string{"legacy_processing_task_id", "legacy_action_key"}, true)

			return nil
		},
	}
}
