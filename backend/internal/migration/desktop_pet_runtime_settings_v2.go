// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeSettingsV2Migration() Migration {
	return Migration{
		Version: "202607310004",
		Name:    "add_desktop_pet_runtime_settings_v2_fields",
		Up: func(s *Step) error {
			cols := [][2]string{
				{"settings_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"restore_on_app_start", "INTEGER NOT NULL DEFAULT 1"},
				{"position_mode", "TEXT NOT NULL DEFAULT 'absolute'"},
				{"display_fingerprint", "TEXT NOT NULL DEFAULT ''"},
				{"relative_x", "REAL NOT NULL DEFAULT 0.5"},
				{"relative_y", "REAL NOT NULL DEFAULT 0.5"},
				{"last_window_width", "INTEGER NOT NULL DEFAULT 0"},
				{"last_window_height", "INTEGER NOT NULL DEFAULT 0"},
				{"position_updated_at", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, c := range cols {
				if err := s.AddColumn("desktop_pet_runtime_settings", c[0], c[1]); err != nil {
					return err
				}
			}
			s.Execute("UPDATE desktop_pet_runtime_settings SET click_through_mode='off' WHERE click_through_mode='alpha' AND position_x=0 AND position_y=0 AND screen_id=''")
			s.Execute("UPDATE desktop_pet_runtime_settings SET settings_revision=0 WHERE settings_revision IS NULL")
			return nil
		},
	}
}
