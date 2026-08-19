// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeActualStatesMigration() Migration {
	return Migration{
		Version: "202607310003",
		Name:    "add_desktop_pet_runtime_actual_states_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_actual_states (
  runtime_id TEXT NOT NULL,
  installation_id TEXT NOT NULL DEFAULT '',
  pet_instance_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  desired_revision INTEGER NOT NULL DEFAULT 0,
  applied_settings_revision INTEGER NOT NULL DEFAULT 0,
  visible INTEGER NOT NULL DEFAULT 0,
  current_action_key TEXT NOT NULL DEFAULT '',
  position_x INTEGER NOT NULL DEFAULT 0,
  position_y INTEGER NOT NULL DEFAULT 0,
  screen_id TEXT NOT NULL DEFAULT '',
  scale REAL NOT NULL DEFAULT 1.0,
  health TEXT NOT NULL DEFAULT 'unknown',
  state_json TEXT NOT NULL DEFAULT '{}',
  observed_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY(runtime_id, installation_id)
)`)
			return nil
		},
	}
}
