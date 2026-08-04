// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func RotationJournalMigration() Migration {
	return Migration{
		Version: "202608030003",
		Name:    "add_desktop_pet_token_rotation_journal",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_token_rotation_journal (
				id TEXT PRIMARY KEY,
				old_version TEXT NOT NULL DEFAULT '',
				new_version TEXT NOT NULL DEFAULT '',
				stage TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				completed_at TEXT NOT NULL DEFAULT ''
			)`)
			if err := s.CreateIndex("idx_dptrj_stage", "desktop_pet_token_rotation_journal", []string{"stage"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
