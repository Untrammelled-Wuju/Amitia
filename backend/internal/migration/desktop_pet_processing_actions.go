// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetProcessingActionsMigration() Migration {
	return Migration{
		Version:           "202607240010",
		Name:              "add_desktop_pet_processing_actions_table",
		AcceptedChecksums: []string{"b7eb16662292998f147104dbe18dfad658659aeaca580a84e2c7eb084a86ff81"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_actions (
id TEXT PRIMARY KEY,
processing_task_id TEXT NOT NULL DEFAULT '',
generation_task_action_id TEXT NOT NULL DEFAULT '',
action_key TEXT NOT NULL DEFAULT '',
action_name_snapshot TEXT DEFAULT '',
source_attempt_number INTEGER NOT NULL DEFAULT 1,
status TEXT NOT NULL DEFAULT 'pending',
progress INTEGER NOT NULL DEFAULT 0,
source_frame_count INTEGER NOT NULL DEFAULT 0,
processed_frame_count INTEGER NOT NULL DEFAULT 0,
loop_type TEXT DEFAULT 'once',
fps INTEGER NOT NULL DEFAULT 10,
frame_duration_ms INTEGER NOT NULL DEFAULT 100,
anchor_type TEXT DEFAULT 'feet_center',
anchor_x REAL NOT NULL DEFAULT 0.5,
anchor_y REAL NOT NULL DEFAULT 0.92,
bounding_box TEXT DEFAULT '',
excluded INTEGER NOT NULL DEFAULT 0,
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT '',
started_at TEXT DEFAULT '',
completed_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dppa_task", "desktop_pet_processing_actions", []string{"processing_task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppa_action", "desktop_pet_processing_actions", []string{"action_key"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppa_status", "desktop_pet_processing_actions", []string{"status"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
