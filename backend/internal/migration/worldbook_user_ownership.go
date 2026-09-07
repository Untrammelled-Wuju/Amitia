// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// WorldBookUserOwnershipMigration separates account ownership from optional
// character scope. Cloud Core must never use character_id as a tenant key.
func WorldBookUserOwnershipMigration() Migration {
	return Migration{
		Version: "20260907005",
		Name:    "add_worldbook_user_ownership",
		Up: func(s *Step) error {
			if err := s.AddColumn("world_book", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			if err := s.AddColumn("world_book", "character_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			// Character ownership is the strongest attribution signal for scoped
			// rules. Global legacy rules remain with their existing/default owner
			// instead of being assigned to an arbitrary cloud account.
			s.Execute(`
				UPDATE world_book
				SET user_id = COALESCE(
					(SELECT c.user_id FROM characters c WHERE c.id = world_book.character_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
					NULLIF(TRIM(user_id), ''),
					'default'
				)
				WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'
			`)
			s.Execute("CREATE INDEX IF NOT EXISTS idx_world_book_user_character ON world_book(user_id, character_id)")
			s.Execute("CREATE INDEX IF NOT EXISTS idx_world_book_user_priority ON world_book(user_id, priority)")
			return nil
		},
	}
}
