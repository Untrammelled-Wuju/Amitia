// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetGenerationTasksMigration() Migration {
	return Migration{
		Version: "202607240004",
		Name:    "add_desktop_pet_generation_tasks_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_tasks (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
model_config_id INTEGER NOT NULL DEFAULT 0,
name TEXT NOT NULL DEFAULT '',
source_image_path TEXT DEFAULT '',
source_image_original_name TEXT DEFAULT '',
source_image_mime_type TEXT DEFAULT '',
source_image_size INTEGER DEFAULT 0,
source_image_width INTEGER DEFAULT 0,
source_image_height INTEGER DEFAULT 0,
source_image_hash TEXT DEFAULT '',
prompt TEXT DEFAULT '',
negative_prompt TEXT DEFAULT '',
output_width INTEGER DEFAULT 0,
output_height INTEGER DEFAULT 0,
status TEXT NOT NULL DEFAULT 'pending',
current_stage TEXT NOT NULL DEFAULT 'queued',
progress INTEGER NOT NULL DEFAULT 0,
selected_action_count INTEGER NOT NULL DEFAULT 0,
estimated_generation_count INTEGER NOT NULL DEFAULT 0,
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT (datetime('now')),
updated_at TEXT DEFAULT (datetime('now')),
started_at TEXT DEFAULT '',
completed_at TEXT DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_task_actions (
id TEXT PRIMARY KEY,
task_id TEXT NOT NULL DEFAULT '',
action_definition_id INTEGER NOT NULL DEFAULT 0,
action_key TEXT NOT NULL DEFAULT '',
action_name_snapshot TEXT DEFAULT '',
action_description_snapshot TEXT DEFAULT '',
category_key_snapshot TEXT DEFAULT '',
category_name_snapshot TEXT DEFAULT '',
definition_version INTEGER NOT NULL DEFAULT 1,
supports_default_idle INTEGER NOT NULL DEFAULT 0,
sort_order INTEGER NOT NULL DEFAULT 0,
frame_count INTEGER NOT NULL DEFAULT 8,
estimated_generation_count INTEGER NOT NULL DEFAULT 1,
status TEXT NOT NULL DEFAULT 'pending',
progress INTEGER NOT NULL DEFAULT 0,
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT (datetime('now')),
updated_at TEXT DEFAULT (datetime('now')),
started_at TEXT DEFAULT '',
completed_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dpgt_user", "desktop_pet_generation_tasks", []string{"user_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgt_char", "desktop_pet_generation_tasks", []string{"character_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgt_status", "desktop_pet_generation_tasks", []string{"status"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgt_created", "desktop_pet_generation_tasks", []string{"created_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgta_task", "desktop_pet_generation_task_actions", []string{"task_id"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
