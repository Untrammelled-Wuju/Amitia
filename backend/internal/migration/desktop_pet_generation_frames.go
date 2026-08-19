// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetGenerationFramesMigration() Migration {
	return Migration{
		Version:           "202607240007",
		Name:              "add_desktop_pet_generation_frames_table",
		AcceptedChecksums: []string{"bffc083ef1fe063c267db20517cda0f2084a9d5ee67e94f8ece2917339404e27"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_generation_frames (
id TEXT PRIMARY KEY,
task_id TEXT NOT NULL DEFAULT '',
task_action_id TEXT NOT NULL DEFAULT '',
execution_id TEXT DEFAULT '',
frame_index INTEGER NOT NULL DEFAULT 0,
frame_phase TEXT DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
attempt_number INTEGER NOT NULL DEFAULT 1,
prompt_snapshot TEXT DEFAULT '',
negative_prompt_snapshot TEXT DEFAULT '',
provider TEXT DEFAULT '',
model TEXT DEFAULT '',
provider_request_id TEXT DEFAULT '',
provider_operation_id TEXT DEFAULT '',
source_image_path TEXT DEFAULT '',
previous_frame_path TEXT DEFAULT '',
result_image_path TEXT DEFAULT '',
result_mime_type TEXT DEFAULT '',
result_width INTEGER DEFAULT 0,
result_height INTEGER DEFAULT 0,
result_size INTEGER DEFAULT 0,
result_hash TEXT DEFAULT '',
provider_seed TEXT DEFAULT '',
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT '',
started_at TEXT DEFAULT '',
completed_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dpgf_task", "desktop_pet_generation_frames", []string{"task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgf_action", "desktop_pet_generation_frames", []string{"task_action_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgf_exec", "desktop_pet_generation_frames", []string{"execution_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dpgf_status", "desktop_pet_generation_frames", []string{"status"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
