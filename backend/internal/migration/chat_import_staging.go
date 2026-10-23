package migration

func ChatImportStagingMigration() Migration {
	return Migration{
		Version: "20261023005",
		Name:    "chat_import_staging",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS chat_import_batches (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				device_id TEXT NOT NULL DEFAULT '',
				role_id TEXT NOT NULL DEFAULT '',
				role_revision INTEGER NOT NULL DEFAULT 0,
				title TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'ready_for_promotion',
				detected_format TEXT NOT NULL DEFAULT '',
				message_count INTEGER NOT NULL DEFAULT 0,
				content_hash TEXT NOT NULL DEFAULT '',
				idempotency_key TEXT NOT NULL DEFAULT '',
				summary_text TEXT NOT NULL DEFAULT '',
				promoted_conversation_id TEXT NOT NULL DEFAULT '',
				promoted_conversation_owner_id TEXT NOT NULL DEFAULT '',
				promoted_core_id TEXT NOT NULL DEFAULT '',
				legacy_conversation_id TEXT NOT NULL DEFAULT '',
				error_code TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				promoted_at TEXT NOT NULL DEFAULT ''
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS chat_import_items (
				id TEXT PRIMARY KEY,
				batch_id TEXT NOT NULL,
				sequence INTEGER NOT NULL DEFAULT 0,
				speaker TEXT NOT NULL DEFAULT '',
				role TEXT NOT NULL DEFAULT 'user',
				content TEXT NOT NULL,
				confidence REAL NOT NULL DEFAULT 0,
				timestamp TEXT NOT NULL DEFAULT '',
				line_no INTEGER NOT NULL DEFAULT 0,
				created_at TEXT NOT NULL DEFAULT ''
			)`)
			s.CreateIndex("idx_chat_import_batches_space_status", "chat_import_batches", []string{"space_id", "status", "created_at"}, false)
			s.CreateIndex("idx_chat_import_batches_idempotency", "chat_import_batches", []string{"space_id", "device_id", "idempotency_key"}, true)
			s.CreateIndex("idx_chat_import_batches_promoted", "chat_import_batches", []string{"space_id", "promoted_conversation_id"}, false)
			s.CreateIndex("idx_chat_import_items_batch_sequence", "chat_import_items", []string{"batch_id", "sequence"}, true)
			s.Execute(`INSERT OR IGNORE INTO chat_import_batches (
				id, space_id, device_id, role_id, role_revision, title, status,
				detected_format, message_count, content_hash, idempotency_key, summary_text,
				promoted_conversation_id, promoted_conversation_owner_id,
				promoted_core_id, legacy_conversation_id, error_code,
				error_message, created_at, updated_at, promoted_at
			)
			SELECT
				'legacy-import-' || c.id,
				COALESCE(c.space_id, ''),
				'',
				COALESCE((SELECT m.character_id FROM messages m WHERE m.conversation_id = c.id AND m.character_id <> '' ORDER BY m.sequence ASC, m.id ASC LIMIT 1), ''),
				0,
				COALESCE(c.title, '已导入的聊天'),
				CASE WHEN EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.character_id <> '') THEN 'ready_for_promotion' ELSE 'failed' END,
				'',
				(SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id AND m.deleted_at IS NULL),
				'',
				'legacy-conversation:' || c.id,
				'',
				'',
				'',
				'',
				c.id,
				CASE WHEN EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.character_id <> '') THEN '' ELSE 'IMPORT_ROLE_MISSING' END,
				CASE WHEN EXISTS (SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.character_id <> '') THEN '' ELSE '导入会话缺少角色归属' END,
				COALESCE(c.created_at, ''),
				COALESCE(c.updated_at, c.created_at, ''),
				''
			FROM conversations c
			WHERE COALESCE(c.source, '') = 'import' AND c.deleted_at IS NULL`)
			s.Execute(`INSERT OR IGNORE INTO chat_import_items (
				id, batch_id, sequence, speaker, role, content, confidence,
				timestamp, line_no, created_at
			)
			SELECT
				'legacy-import-item-' || m.id,
				b.id,
				m.sequence,
				'',
				COALESCE(NULLIF(m.role, ''), 'user'),
				m.content,
				1,
				'',
				m.sequence,
				COALESCE(m.created_at, b.created_at, '')
			FROM messages m
			JOIN conversations c ON c.id = m.conversation_id
			JOIN chat_import_batches b ON b.legacy_conversation_id = c.id
			WHERE COALESCE(c.source, '') = 'import' AND c.deleted_at IS NULL AND m.deleted_at IS NULL`)
			return nil
		},
	}
}
