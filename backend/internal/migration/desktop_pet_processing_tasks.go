// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetProcessingTasksMigration() Migration {
	return Migration{
		Version: "202607240009",
		Name:    "add_desktop_pet_processing_tasks_table",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_tasks (
id TEXT PRIMARY KEY,
generation_task_id TEXT NOT NULL DEFAULT '',
processing_version INTEGER NOT NULL DEFAULT 1,
status TEXT NOT NULL DEFAULT 'pending',
current_stage TEXT NOT NULL DEFAULT 'queued',
progress INTEGER NOT NULL DEFAULT 0,
output_width INTEGER NOT NULL DEFAULT 512,
output_height INTEGER NOT NULL DEFAULT 512,
target_character_height_ratio REAL NOT NULL DEFAULT 0.8,
anchor_mode TEXT NOT NULL DEFAULT 'feet_center',
background_mode TEXT NOT NULL DEFAULT 'remove_background',
output_format TEXT NOT NULL DEFAULT 'png',
default_fps INTEGER NOT NULL DEFAULT 10,
execution_id TEXT DEFAULT '',
worker_id TEXT DEFAULT '',
lease_expires_at TEXT DEFAULT '',
last_heartbeat_at TEXT DEFAULT '',
cancel_requested_at TEXT DEFAULT '',
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
started_at TEXT DEFAULT '',
completed_at TEXT DEFAULT '',
created_at TEXT DEFAULT (datetime('now')),
updated_at TEXT DEFAULT (datetime('now'))
)`)
			if err := s.CreateIndex("idx_dppt_gen_task", "desktop_pet_processing_tasks", []string{"generation_task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppt_status", "desktop_pet_processing_tasks", []string{"status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppt_version", "desktop_pet_processing_tasks", []string{"processing_version"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppt_exec", "desktop_pet_processing_tasks", []string{"execution_id"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
