package migration

func SidebarProjectsMigration() Migration {
	return Migration{
		Version: "20260919001",
		Name:    "add_projects_and_unify_conversation_workspace",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS projects (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL DEFAULT '',
				name TEXT NOT NULL,
				workspace_id TEXT NOT NULL,
				device_id TEXT NOT NULL DEFAULT '',
				root_uri TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				revision INTEGER NOT NULL DEFAULT 1,
				UNIQUE(space_id, workspace_id)
			)`)
			if err := s.AddColumn("messages", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute(`UPDATE messages
				SET character_id = COALESCE((
					SELECT c.character_id FROM conversations c WHERE c.id = messages.conversation_id
				), '')
				WHERE COALESCE(character_id, '') = ''`)
			s.Execute(`INSERT OR IGNORE INTO projects (id, space_id, name, workspace_id, device_id, root_uri, created_at, updated_at, revision)
				SELECT
					'project-' || lower(hex(randomblob(16))),
					COALESCE(NULLIF(c.space_id, ''), 'default'),
					COALESCE(NULLIF(b.workspace_name, ''), NULLIF(w.name, ''), '项目'),
					b.workspace_id,
					COALESCE(b.device_id, ''),
					COALESCE(NULLIF(b.root_uri, ''), 'amitia://workspace/@' || b.workspace_id || '/'),
					COALESCE(NULLIF(c.created_at, ''), datetime('now')),
					COALESCE(NULLIF(c.updated_at, ''), datetime('now')),
					1
				FROM conversation_workspace_bindings b
				JOIN conversations c ON c.id = b.conversation_id
				LEFT JOIN workspace_mounts w ON w.id = b.workspace_id
				GROUP BY COALESCE(NULLIF(c.space_id, ''), 'default'), b.workspace_id`)
			s.CreateTable(`CREATE TABLE conversations_v2 (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL DEFAULT '',
				project_id TEXT NOT NULL DEFAULT '',
				title TEXT DEFAULT '',
				channel TEXT DEFAULT 'web',
				source TEXT DEFAULT 'manual',
				peer_id TEXT DEFAULT '',
				message_count INTEGER DEFAULT 0,
				state_version TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT '',
				revision INTEGER NOT NULL DEFAULT 1,
				deleted_at DATETIME
			)`)
			s.Execute(`INSERT INTO conversations_v2 (id, space_id, project_id, title, channel, source, peer_id, message_count, state_version, created_at, updated_at, revision, deleted_at)
				SELECT
					c.id,
					c.space_id,
					COALESCE((
						SELECT p.id
						FROM projects p
						JOIN conversation_workspace_bindings b ON b.workspace_id = p.workspace_id
						WHERE b.conversation_id = c.id AND p.space_id = COALESCE(NULLIF(c.space_id, ''), 'default')
						LIMIT 1
					), ''),
					c.title, c.channel, c.source, c.peer_id, c.message_count, c.state_version,
					c.created_at, c.updated_at, c.revision, c.deleted_at
				FROM conversations c`)
			s.Execute("DROP INDEX IF EXISTS idx_conversations_user_character")
			s.Execute("DROP INDEX IF EXISTS idx_conversations_character")
			s.Execute("DROP INDEX IF EXISTS idx_conversations_character_channel_updated")
			s.Execute("DROP INDEX IF EXISTS idx_conversations_character_updated")
			s.Execute("DROP INDEX IF EXISTS idx_conversations_character_id")
			s.Execute("DROP TABLE conversations")
			s.Execute("ALTER TABLE conversations_v2 RENAME TO conversations")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_conversations_user_updated ON conversations(space_id, updated_at)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_conversations_project_updated ON conversations(project_id, updated_at)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_conversations_channel_peer ON conversations(channel, peer_id)")
			s.Execute("CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_channel_peer_unique ON conversations(channel, peer_id) WHERE peer_id <> ''")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_messages_character_id ON messages(character_id)")
			s.Execute("DROP TABLE IF EXISTS conversation_workspace_bindings")
			s.Execute("UPDATE characters SET conversation_id = ''")
			return nil
		},
	}
}
