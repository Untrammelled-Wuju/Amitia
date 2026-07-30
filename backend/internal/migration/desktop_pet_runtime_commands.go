// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeCommandsMigration() Migration {
	return Migration{
		Version: "202607310002",
		Name:    "add_desktop_pet_runtime_commands_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_commands (
  id TEXT PRIMARY KEY,
  runtime_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  installation_id TEXT NOT NULL DEFAULT '',
  pet_instance_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  durability TEXT NOT NULL DEFAULT 'durable',
  coalesce_key TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  desired_revision INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  next_attempt_at TEXT NOT NULL DEFAULT '',
  deadline_at TEXT NOT NULL DEFAULT '',
  last_session_id TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error_message TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dprcmd_idempotency", "desktop_pet_runtime_commands", []string{"runtime_id", "idempotency_key"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dprcmd_dispatch", "desktop_pet_runtime_commands", []string{"runtime_id", "status", "next_attempt_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dprcmd_coalesce", "desktop_pet_runtime_commands", []string{"runtime_id", "coalesce_key", "desired_revision"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
