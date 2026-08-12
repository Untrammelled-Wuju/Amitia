// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ModelConfigProtocolMigration() Migration {
	return Migration{
		Version: "202608120001",
		Name:    "add_protocol_and_message_attachments",
		Up: func(s *Step) error {
			if err := s.AddColumn("model_configs", "protocol", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("model_configs", "context_window", "INTEGER DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("model_configs", "max_output_tokens", "INTEGER DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("model_configs", "capabilities_json", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			s.CreateTable(`CREATE TABLE IF NOT EXISTS message_attachments (
				id TEXT PRIMARY KEY,
				message_id TEXT NOT NULL DEFAULT '',
				sequence INTEGER NOT NULL DEFAULT 0,
				type TEXT NOT NULL DEFAULT '',
				resource_uri TEXT NOT NULL DEFAULT '',
				mime_type TEXT DEFAULT '',
				filename TEXT DEFAULT '',
				size_bytes INTEGER DEFAULT 0,
				content_hash TEXT DEFAULT '',
				width INTEGER DEFAULT 0,
				height INTEGER DEFAULT 0,
				duration_ms INTEGER DEFAULT 0,
				created_at TEXT DEFAULT ''
			)`)
			s.CreateIndex("idx_message_attachments_message_id", "message_attachments", []string{"message_id"}, false)
			return nil
		},
	}
}
