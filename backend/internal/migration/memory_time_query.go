// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func MemoryTimeQueryIndexesMigration() Migration {
	return Migration{
		Version: "20260813001",
		Name:    "memory_time_query_indexes",
		Up: func(s *Step) error {
			if err := s.CreateIndex("idx_memories_character_type", "memories", []string{"character_id", "memory_type"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_memory_temporal_validity", "memory_temporal_metadata", []string{"valid_from_utc", "valid_to_utc"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_memory_temporal_local_date", "memory_temporal_metadata", []string{"local_date"}, false); err != nil {
				return err
			}
			return nil
		},
	}
}
