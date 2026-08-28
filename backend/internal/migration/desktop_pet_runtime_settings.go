// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeSettingsMigration() Migration {
	return Migration{
		Version: "202607250002",
		Name:    "add_desktop_pet_runtime_settings_table",
		AcceptedChecksums: []string{
			"79a8a8f2d904cbdcd1b2cd09ce39269e1c2f16ffe20c22c8c59b17301a8c9414",
			"8b41581f4fa31b714003a792900173fe363275378ff744ea0c5fe90160bf68c1",
		},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_settings (
id TEXT PRIMARY KEY,
installation_id TEXT NOT NULL DEFAULT '',
always_on_top INTEGER NOT NULL DEFAULT 1,
launch_on_startup INTEGER NOT NULL DEFAULT 0,
scale REAL NOT NULL DEFAULT 1.0,
position_x INTEGER NOT NULL DEFAULT 0,
position_y INTEGER NOT NULL DEFAULT 0,
screen_id TEXT NOT NULL DEFAULT '',
idle_enabled INTEGER NOT NULL DEFAULT 1,
idle_interval_min_seconds INTEGER NOT NULL DEFAULT 30,
idle_interval_max_seconds INTEGER NOT NULL DEFAULT 120,
click_through_mode TEXT NOT NULL DEFAULT 'alpha',
sound_enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dprts_installation", "desktop_pet_runtime_settings", []string{"installation_id"}, true); err != nil {
				return err
			}
			return nil
		},
	}
}
