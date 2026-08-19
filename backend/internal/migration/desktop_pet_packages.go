// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetPackagesMigration() Migration {
	return Migration{
		Version:           "202607240012",
		Name:              "add_desktop_pet_packages_table",
		AcceptedChecksums: []string{
			"4d608d1bfbedb38bef7b10d47d15fe82d8d7a26183dc59df5f9d1210dc52d0c8",
			"0a7b177111aa0ebb2972d160b6c5fd6affa982f0a8f91c563a627f7b78ad1195",
		},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_packages (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
generation_task_id TEXT NOT NULL DEFAULT '',
processing_task_id TEXT NOT NULL DEFAULT '',
name TEXT NOT NULL DEFAULT '',
version INTEGER NOT NULL DEFAULT 1,
status TEXT NOT NULL DEFAULT 'draft',
default_action_key TEXT NOT NULL DEFAULT '',
canvas_width INTEGER NOT NULL DEFAULT 512,
canvas_height INTEGER NOT NULL DEFAULT 512,
package_path TEXT DEFAULT '',
manifest_path TEXT DEFAULT '',
preview_path TEXT DEFAULT '',
action_count INTEGER NOT NULL DEFAULT 0,
package_hash TEXT DEFAULT '',
included_actions TEXT DEFAULT '[]',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dppkg_user", "desktop_pet_packages", []string{"user_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppkg_gen", "desktop_pet_packages", []string{"generation_task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppkg_proc", "desktop_pet_packages", []string{"processing_task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppkg_status", "desktop_pet_packages", []string{"status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppkg_proc_version", "desktop_pet_packages", []string{"processing_task_id", "version"}, true); err != nil {
				return err
			}
			return nil
		},
	}
}
