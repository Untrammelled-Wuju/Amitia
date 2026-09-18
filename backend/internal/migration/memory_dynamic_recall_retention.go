// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func MemoryDynamicRecallRetentionMigration() Migration {
	return Migration{
		Version: "20260903004",
		Name:    "memory_dynamic_recall_retention",
		Up: func(s *Step) error {
			s.AddColumn("memories", "memory_subtype", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memories", "retention_level", "INTEGER NOT NULL DEFAULT 3")
			s.AddColumn("memories", "memory_strength", "REAL NOT NULL DEFAULT 0.68")
			s.AddColumn("memories", "strength_updated_at", "TEXT DEFAULT NULL")
			s.AddColumn("memories", "last_reinforced_at", "TEXT DEFAULT NULL")
			s.AddColumn("memories", "reinforce_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memories", "retrieved_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memories", "injected_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memories", "decay_state", "TEXT NOT NULL DEFAULT 'active'")
			s.AddColumn("memories", "pinned", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memories", "archived_at", "TEXT DEFAULT NULL")
			s.AddColumn("memories", "superseded_by", "TEXT NOT NULL DEFAULT ''")

			s.AddColumn("memory_candidates", "memory_subtype", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "scope", "TEXT NOT NULL DEFAULT 'character'")
			s.AddColumn("memory_candidates", "retention_level", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memory_candidates", "memory_strength", "REAL NOT NULL DEFAULT 0")
			s.AddColumn("memory_candidates", "strength_updated_at", "TEXT DEFAULT NULL")
			s.AddColumn("memory_candidates", "last_reinforced_at", "TEXT DEFAULT NULL")
			s.AddColumn("memory_candidates", "reinforce_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memory_candidates", "decay_state", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("memory_candidates", "pinned", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("memory_candidates", "archived_at", "TEXT DEFAULT NULL")

			s.AddColumn("episodic_memories", "retention_level", "INTEGER NOT NULL DEFAULT 4")
			s.AddColumn("episodic_memories", "memory_strength", "REAL NOT NULL DEFAULT 0.50")
			s.AddColumn("episodic_memories", "strength_updated_at", "TEXT DEFAULT NULL")
			s.AddColumn("episodic_memories", "last_reinforced_at", "TEXT DEFAULT NULL")
			s.AddColumn("episodic_memories", "reinforce_count", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("episodic_memories", "decay_state", "TEXT NOT NULL DEFAULT 'active'")
			s.AddColumn("episodic_memories", "archived_at", "TEXT DEFAULT NULL")

			s.AddColumn("user_profiles", "source_memory_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("user_profiles", "projection_status", "TEXT NOT NULL DEFAULT 'active'")

			s.Execute(`UPDATE memories SET
				retention_level = CASE
					WHEN memory_type = 'personal_info' AND importance >= 9 THEN 1
					WHEN importance >= 9 THEN 2
					WHEN importance >= 7 THEN 3
					WHEN importance >= 4 THEN 4
					ELSE 5
				END,
				memory_strength = CASE
					WHEN memory_type = 'personal_info' AND importance >= 9 THEN 1.0
					WHEN importance >= 9 THEN 0.86
					WHEN importance >= 7 THEN 0.68
					WHEN importance >= 4 THEN 0.50
					ELSE 0.36
				END,
				strength_updated_at = COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, '')),
				last_reinforced_at = COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, '')),
				decay_state = 'active'
			WHERE retention_level IS NULL OR retention_level = 3`)
			s.Execute(`UPDATE episodic_memories SET
				retention_level = 4,
				memory_strength = CASE WHEN ABS(sentiment_score) >= 70 THEN 0.62 ELSE 0.50 END,
				strength_updated_at = COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, '')),
				last_reinforced_at = COALESCE(NULLIF(updated_at, ''), NULLIF(created_at, '')),
				decay_state = 'active'
			WHERE retention_level IS NULL OR retention_level = 4`)

			s.CreateIndex("idx_memories_retention_state", "memories", []string{"character_id", "decay_state", "retention_level"}, false)
			s.CreateIndex("idx_memories_subtype", "memories", []string{"character_id", "memory_subtype"}, false)
			s.CreateIndex("idx_episodic_retention_state", "episodic_memories", []string{"user_id", "decay_state", "retention_level"}, false)
			s.CreateIndex("idx_user_profiles_source_memory", "user_profiles", []string{"source_memory_id"}, false)
			s.CreateIndex("idx_user_profiles_projection_status", "user_profiles", []string{"user_id", "character_id", "projection_status"}, false)
			return nil
		},
	}
}
