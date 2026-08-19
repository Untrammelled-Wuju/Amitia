package migration

func DesktopPetRevisionBridgeMigration() Migration {
	return Migration{
		Version: "202608020002",
		Name:    "add_desktop_pet_revision_bridge_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_streams (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  next_revision_number INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_das_user_char_action", "desktop_pet_action_streams", []string{"user_id", "character_id", "action_key"}, true)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_revision_bridge_journals (
  id TEXT PRIMARY KEY,
  processing_revision_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  target_action_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'processing_published',
  last_error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
			s.CreateIndex("idx_drbj_status", "desktop_pet_revision_bridge_journals", []string{"status"}, false)
			s.CreateIndex("idx_drbj_proc_rev", "desktop_pet_revision_bridge_journals", []string{"processing_revision_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_active_action_revision_bindings (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  active_action_revision_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  bound_reason TEXT NOT NULL DEFAULT '',
  bound_by TEXT NOT NULL DEFAULT '',
  bound_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
			s.CreateIndex("uq_daarb_user_char_action", "desktop_pet_active_action_revision_bindings", []string{"user_id", "character_id", "action_key"}, true)

			if err := s.AddColumn("desktop_pet_action_revisions", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "source_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "source_processing_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "content_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "content_hash_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "action_config_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "frame_set_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "origin", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "playback_mode", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "anchor_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_revisions", "archived_reason", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			s.CreateIndex("uq_dpar_source_type", "desktop_pet_action_revisions", []string{"source_processing_revision_id", "source_type"}, false)

			if err := s.AddColumn("desktop_pet_action_active_revisions", "user_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "active_action_revision_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "binding_revision", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "bound_reason", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "bound_by", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_action_active_revisions", "bound_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			return nil
		},
	}
}
