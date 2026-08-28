// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeClientsMigration() Migration {
	return Migration{
		Version:           "202607310001",
		Name:              "add_desktop_pet_runtime_clients_table",
		AcceptedChecksums: []string{"b11e2fcb9bc79fa50f5fd2766edf8cd8cae3a61432dfb6dd069ea3c8661b4a57"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_clients (
  runtime_id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  app_version TEXT NOT NULL DEFAULT '',
  protocol_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  last_process_instance_id TEXT NOT NULL DEFAULT '',
  last_session_id TEXT NOT NULL DEFAULT '',
  last_seen_at TEXT NOT NULL DEFAULT '',
  last_connected_at TEXT NOT NULL DEFAULT '',
  last_disconnected_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT '',
  updated_at TEXT DEFAULT ''
)`)
			return nil
		},
	}
}
