// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetLocalSessionFixMigration() Migration {
	return Migration{
		Version: "202608080001",
		Name:    "fix_desktop_pet_local_sessions_revoked_at_nullable",
		AcceptedChecksums: []string{
			"38de07f93e610bc8278bb66b72fb003509c0b7404860c75ee1c47d0347169dd1",
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		Up: func(s *Step) error {
			exists, err := s.TableExists("desktop_pet_local_sessions")
			if err != nil {
				return err
			}
			if !exists {
				s.Execute("SELECT 1")
				return nil
			}
			var colNotNull bool
			err = s.DB().Raw("SELECT \"notnull\" FROM pragma_table_info('desktop_pet_local_sessions') WHERE name='revoked_at'").Scan(&colNotNull).Error
			if err != nil {
				return err
			}
			if !colNotNull {
				s.Execute("SELECT 1")
				return nil
			}
			s.Execute(`CREATE TABLE IF NOT EXISTS desktop_pet_local_sessions_new (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL DEFAULT '',
				desktop_instance_id TEXT NOT NULL DEFAULT '',
				token_hash TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				created_at TEXT NOT NULL DEFAULT '',
				expires_at TEXT NOT NULL DEFAULT '',
				last_used_at TEXT NOT NULL DEFAULT '',
				revoked_at TEXT DEFAULT NULL
			)`)
			s.Execute(`INSERT INTO desktop_pet_local_sessions_new (id,user_id,desktop_instance_id,token_hash,status,created_at,expires_at,last_used_at,revoked_at) SELECT id,user_id,desktop_instance_id,token_hash,status,created_at,expires_at,last_used_at,CASE WHEN revoked_at='' THEN NULL ELSE revoked_at END FROM desktop_pet_local_sessions`)
			s.Execute(`DROP TABLE IF EXISTS desktop_pet_local_sessions`)
			s.Execute(`ALTER TABLE desktop_pet_local_sessions_new RENAME TO desktop_pet_local_sessions`)
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
