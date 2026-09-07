// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func CharacterUserOwnershipMigration() Migration {
	return Migration{
		Version: "20260907002",
		Name:    "add_character_user_ownership",
		Up: func(s *Step) error {
			db := s.DB()
			if !db.Migrator().HasColumn("characters", "user_id") {
				if err := db.Exec("ALTER TABLE characters ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default'").Error; err != nil {
					return err
				}
			}
			// A character's canonical conversation is the strongest available owner signal.
			if err := db.Exec(`
				UPDATE characters
				SET user_id = COALESCE(
					(SELECT c.user_id FROM conversations c WHERE c.id = characters.conversation_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
					(SELECT sc.user_id FROM sync_changes sc WHERE sc.entity_type = 'character' AND sc.entity_id = characters.id AND TRIM(COALESCE(sc.user_id, '')) <> '' ORDER BY sc.created_at DESC LIMIT 1),
					NULLIF(TRIM(user_id), ''),
					'default'
				)
				WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'
			`).Error; err != nil {
				return err
			}
			if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_characters_user_updated ON characters(user_id, updated_at)").Error; err != nil {
				return err
			}
			if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_characters_user_active ON characters(user_id, is_active, is_default)").Error; err != nil {
				return err
			}
			return nil
		},
	}
}
