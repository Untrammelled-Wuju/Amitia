// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetInstallationsMigration() Migration {
	return Migration{
		Version:           "202607250001",
		Name:              "add_desktop_pet_installations_table",
		AcceptedChecksums: []string{"a398213ea33399de6f69360dddfd8f8c6f13978968308c8c7d20349be1b3f53a"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_installations (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
package_id TEXT NOT NULL DEFAULT '',
package_version TEXT NOT NULL DEFAULT '',
name TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'installed',
is_active INTEGER NOT NULL DEFAULT 0,
install_path TEXT DEFAULT '',
manifest_path TEXT DEFAULT '',
preview_path TEXT DEFAULT '',
default_action_key TEXT DEFAULT '',
canvas_width INTEGER NOT NULL DEFAULT 0,
canvas_height INTEGER NOT NULL DEFAULT 0,
package_hash TEXT DEFAULT '',
installed_at TEXT DEFAULT '',
last_enabled_at TEXT DEFAULT '',
last_disabled_at TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dpinst_user", "desktop_pet_installations", []string{"user_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpinst_character", "desktop_pet_installations", []string{"character_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpinst_package", "desktop_pet_installations", []string{"package_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpinst_status", "desktop_pet_installations", []string{"status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpinst_package_version", "desktop_pet_installations", []string{"package_id", "package_version"}, true); err != nil {
				return err
			}
			return nil
		},
	}
}
