// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetGenerationCallLogsMigration() Migration {
	return Migration{
		Version: "202607240008",
		Name:    "add_desktop_pet_generation_call_logs_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_call_logs (
id TEXT PRIMARY KEY,
task_id TEXT NOT NULL DEFAULT '',
task_action_id TEXT DEFAULT '',
frame_id TEXT DEFAULT '',
execution_id TEXT DEFAULT '',
provider TEXT DEFAULT '',
model TEXT DEFAULT '',
provider_request_id TEXT DEFAULT '',
request_started_at TEXT DEFAULT '',
request_completed_at TEXT DEFAULT '',
request_status TEXT DEFAULT '',
attempt_number INTEGER NOT NULL DEFAULT 0,
usage TEXT DEFAULT '',
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dpgcl_task", "desktop_pet_generation_call_logs", []string{"task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgcl_exec", "desktop_pet_generation_call_logs", []string{"execution_id"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
