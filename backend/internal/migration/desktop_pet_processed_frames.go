// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetProcessedFramesMigration() Migration {
	return Migration{
		Version:           "202607240011",
		Name:              "add_desktop_pet_processed_frames_table",
		AcceptedChecksums: []string{"c285ffd9ff5bd50d36120756b3f4184c5f31e9e737a1281fbde97ebcd1628890"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processed_frames (
id TEXT PRIMARY KEY,
processing_action_id TEXT NOT NULL DEFAULT '',
source_frame_id TEXT NOT NULL DEFAULT '',
frame_index INTEGER NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'pending',
source_path TEXT DEFAULT '',
processed_path TEXT DEFAULT '',
width INTEGER DEFAULT 0,
height INTEGER DEFAULT 0,
content_hash TEXT DEFAULT '',
subject_box TEXT DEFAULT '',
anchor_x REAL DEFAULT 0,
anchor_y REAL DEFAULT 0,
alpha_coverage REAL DEFAULT 0,
quality_flags TEXT DEFAULT '',
error_code TEXT DEFAULT '',
error_message TEXT DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`)
			if err := s.CreateIndex("idx_dppf_action", "desktop_pet_processed_frames", []string{"processing_action_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppf_index", "desktop_pet_processed_frames", []string{"frame_index"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppf_status", "desktop_pet_processed_frames", []string{"status"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
