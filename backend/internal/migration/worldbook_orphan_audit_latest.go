package migration

func WorldBookOrphanAuditLatestMigration() Migration {
	return Migration{
		Version: "20261020001",
		Name:    "worldbook_unowned_orphan_audit_latest_additive",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS world_book_orphans (
                id TEXT PRIMARY KEY,
                space_id TEXT NOT NULL DEFAULT '',
                legacy_id TEXT NOT NULL,
                match_type TEXT NOT NULL DEFAULT 'keyword',
                match_pattern TEXT NOT NULL DEFAULT '',
                match_scope TEXT NOT NULL DEFAULT 'full_context',
                inject_content TEXT NOT NULL DEFAULT '',
                priority INTEGER NOT NULL DEFAULT 0,
                hit_count INTEGER NOT NULL DEFAULT 0,
                config_json TEXT NOT NULL DEFAULT '{}',
                migrated_to TEXT NOT NULL DEFAULT '',
                status TEXT NOT NULL DEFAULT 'orphan',
                created_at TEXT NOT NULL DEFAULT '',
                updated_at TEXT NOT NULL DEFAULT ''
            )`)
			s.CreateIndex("idx_world_book_orphan_space_status", "world_book_orphans", []string{"space_id", "status"}, false)
			s.Execute(`INSERT OR IGNORE INTO world_book_orphans
                (id,space_id,legacy_id,match_type,match_pattern,match_scope,inject_content,priority,hit_count,config_json,status,created_at,updated_at)
                SELECT 'legacy:' || id, COALESCE(space_id,''), id, COALESCE(match_type,'keyword'),
                    COALESCE(match_pattern,''), COALESCE(match_scope,'full_context'),COALESCE(inject_content,''),
                    COALESCE(priority,0),COALESCE(hit_count,0),COALESCE(config_json,'{}'),
                    'orphan',COALESCE(created_at,''),COALESCE(updated_at,'')
                FROM world_book WHERE TRIM(COALESCE(character_id,''))=''`)
			return nil
		},
	}
}
