package migration

func CharacterCardImportOperationsMigration() Migration {
	return Migration{
		Version: "20261021003",
		Name:    "character_card_import_operations",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS character_card_import_operations (
				space_id TEXT NOT NULL,
				id TEXT NOT NULL,
				source_hash TEXT NOT NULL,
				character_id TEXT NOT NULL,
				character_name TEXT NOT NULL DEFAULT '',
				source_format TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'pending',
				worldbook_expected INTEGER NOT NULL DEFAULT 0,
				worldbook_imported INTEGER NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				PRIMARY KEY(space_id, id)
			)`)
			s.CreateIndex("idx_character_card_import_source", "character_card_import_operations", []string{"space_id", "source_hash"}, false)
			s.CreateIndex("idx_character_card_import_character", "character_card_import_operations", []string{"space_id", "character_id"}, true)
			return nil
		},
	}
}
