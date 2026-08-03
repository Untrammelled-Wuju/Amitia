// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetLocalSessionMigration() Migration {
	return Migration{
		Version: "202608030001",
		Name:    "add_desktop_pet_local_sessions",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_local_sessions (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL DEFAULT '',
				desktop_instance_id TEXT NOT NULL DEFAULT '',
				token_hash TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				created_at TEXT NOT NULL DEFAULT '',
				expires_at TEXT NOT NULL DEFAULT '',
				last_used_at TEXT NOT NULL DEFAULT '',
				revoked_at TEXT NOT NULL DEFAULT ''
			)`)
			if err := s.CreateIndex("idx_dpls_token", "desktop_pet_local_sessions", []string{"token_hash", "status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpls_user", "desktop_pet_local_sessions", []string{"user_id", "status"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
