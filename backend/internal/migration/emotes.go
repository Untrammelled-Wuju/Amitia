package migration

func EmotesMigration() Migration {
	return Migration{
		Version: "202607180001",
		Name:    "add_emote_mvp",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emotes (
id TEXT PRIMARY KEY,
name TEXT NOT NULL DEFAULT '',
meaning TEXT NOT NULL DEFAULT '',
keywords TEXT NOT NULL DEFAULT '[]',
original_filename TEXT NOT NULL DEFAULT '',
file_path TEXT NOT NULL DEFAULT '',
thumbnail_path TEXT NOT NULL DEFAULT '',
fallback_path TEXT NOT NULL DEFAULT '',
mime_type TEXT NOT NULL DEFAULT '',
file_extension TEXT NOT NULL DEFAULT '',
file_size INTEGER NOT NULL DEFAULT 0,
width INTEGER NOT NULL DEFAULT 0,
height INTEGER NOT NULL DEFAULT 0,
is_animated INTEGER NOT NULL DEFAULT 0,
duration_ms INTEGER NOT NULL DEFAULT 0,
frame_count INTEGER NOT NULL DEFAULT 1,
file_hash TEXT NOT NULL,
enabled INTEGER NOT NULL DEFAULT 1,
ai_enabled INTEGER NOT NULL DEFAULT 0,
role_scope TEXT NOT NULL DEFAULT 'all_characters',
vector_status TEXT NOT NULL DEFAULT 'disabled',
vector_error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
deleted_at TEXT
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emote_groups (
id TEXT PRIMARY KEY,
name TEXT NOT NULL,
cover_emote_id TEXT,
sort_order INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
FOREIGN KEY (cover_emote_id) REFERENCES emotes(id) ON DELETE SET NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emote_group_items (
group_id TEXT NOT NULL,
emote_id TEXT NOT NULL,
sort_order INTEGER NOT NULL DEFAULT 0,
PRIMARY KEY (group_id, emote_id),
FOREIGN KEY (group_id) REFERENCES emote_groups(id) ON DELETE CASCADE,
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE CASCADE
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emote_character_bindings (
emote_id TEXT NOT NULL,
character_id TEXT NOT NULL,
PRIMARY KEY (emote_id, character_id),
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE CASCADE,
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS character_emote_settings (
character_id TEXT PRIMARY KEY,
enabled INTEGER NOT NULL DEFAULT 1,
base_probability REAL NOT NULL DEFAULT 0.10,
max_probability REAL NOT NULL DEFAULT 0.30,
max_per_hour INTEGER NOT NULL DEFAULT 5,
min_reply_gap INTEGER NOT NULL DEFAULT 3,
same_emote_cooldown_minutes INTEGER NOT NULL DEFAULT 30,
allow_emote_only INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS emote_send_records (
id TEXT PRIMARY KEY,
emote_id TEXT,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
message_id TEXT NOT NULL DEFAULT '',
response_id TEXT NOT NULL DEFAULT '',
platform TEXT NOT NULL DEFAULT '',
trigger_type TEXT NOT NULL DEFAULT '',
trigger_probability REAL NOT NULL DEFAULT 0,
random_sample REAL NOT NULL DEFAULT 0,
trigger_hit INTEGER NOT NULL DEFAULT 0,
decision_reason TEXT NOT NULL DEFAULT '',
send_mode TEXT NOT NULL DEFAULT '',
delivery_key TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT '',
failure_reason TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
sent_at TEXT,
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE SET NULL,
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE,
FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
)`)
			if err := s.AddColumn("messages", "emote_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "alt_text", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "is_animated", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "media_width", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "media_height", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "original_asset_reference", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "fallback_asset_reference", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "response_group_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "delivery_sequence", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("messages", "emote_decision_status", "TEXT NOT NULL DEFAULT 'none'"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_emotes_file_hash", "emotes", []string{"file_hash"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_emotes_ai_enabled", "emotes", []string{"enabled", "ai_enabled", "deleted_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_emote_send_character_created", "emote_send_records", []string{"character_id", "created_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_emote_send_emote_created", "emote_send_records", []string{"emote_id", "created_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_emote_send_response", "emote_send_records", []string{"response_id"}, true); err != nil {
				return err
			}
			s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS idx_emote_send_delivery ON emote_send_records(delivery_key) WHERE delivery_key <> ''")
			if err := s.CreateIndex("idx_messages_response_group", "messages", []string{"response_group_id", "delivery_sequence"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
