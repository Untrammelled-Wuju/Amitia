// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func MemoryUserOwnershipMigration() Migration {
	return Migration{
		Version: "20260907003",
		Name:    "memory_user_ownership_scope",
		Up: func(s *Step) error {
			if err := s.AddColumn("memories", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			if err := s.AddColumn("memory_candidates", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			if err := s.AddColumn("episodic_memories", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("retrieval_logs", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}

			// Backfill account ownership from the strongest relational signals first.
			// The old schema sometimes stored character_id in user_id for profile/
			// episodic rows, so that legacy value is only retained when it is not the
			// same as the character scope.
			s.Execute(`UPDATE memories
SET user_id = COALESCE(
    (SELECT c.user_id FROM conversations c WHERE c.id = memories.source_conv_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
    (SELECT ch.user_id FROM characters ch WHERE ch.id = memories.character_id AND TRIM(COALESCE(ch.user_id, '')) <> '' LIMIT 1),
    NULLIF(CASE WHEN TRIM(COALESCE(user_id, '')) <> TRIM(COALESCE(character_id, '')) THEN TRIM(user_id) ELSE '' END, ''),
    'default'
) WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default' OR user_id = character_id`)

			s.Execute(`UPDATE memory_candidates
SET user_id = COALESCE(
    (SELECT c.user_id FROM conversations c WHERE c.id = memory_candidates.conversation_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
    (SELECT ch.user_id FROM characters ch WHERE ch.id = memory_candidates.character_id AND TRIM(COALESCE(ch.user_id, '')) <> '' LIMIT 1),
    NULLIF(TRIM(user_id), ''),
    'default'
) WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`)

			s.Execute(`UPDATE episodic_memories
SET character_id = user_id
WHERE (character_id IS NULL OR TRIM(character_id) = '')
  AND EXISTS (SELECT 1 FROM characters ch WHERE ch.id = episodic_memories.user_id)`)
			s.Execute(`UPDATE episodic_memories
SET user_id = COALESCE(
    (SELECT c.user_id FROM conversations c WHERE c.id = episodic_memories.source_conv_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
    (SELECT ch.user_id FROM characters ch WHERE ch.id = episodic_memories.character_id AND TRIM(COALESCE(ch.user_id, '')) <> '' LIMIT 1),
    NULLIF(CASE WHEN TRIM(COALESCE(user_id, '')) <> TRIM(COALESCE(character_id, '')) THEN TRIM(user_id) ELSE '' END, ''),
    'default'
) WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default' OR user_id = character_id`)

			s.Execute(`UPDATE retrieval_logs
SET user_id = COALESCE(
    (SELECT c.user_id FROM conversations c WHERE c.id = retrieval_logs.conversation_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
    (SELECT ch.user_id FROM characters ch WHERE ch.id = retrieval_logs.character_id AND TRIM(COALESCE(ch.user_id, '')) <> '' LIMIT 1),
    NULLIF(TRIM(user_id), ''),
    'default'
) WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`)

			s.Execute(`UPDATE user_profiles
SET user_id = COALESCE(
    (SELECT c.user_id FROM conversations c WHERE c.id = user_profiles.source_conv_id AND TRIM(COALESCE(c.user_id, '')) <> '' LIMIT 1),
    (SELECT ch.user_id FROM characters ch WHERE ch.id = user_profiles.character_id AND TRIM(COALESCE(ch.user_id, '')) <> '' LIMIT 1),
    NULLIF(CASE WHEN TRIM(COALESCE(user_id, '')) <> TRIM(COALESCE(character_id, '')) THEN TRIM(user_id) ELSE '' END, ''),
    'default'
) WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default' OR user_id = character_id`)

			if err := s.CreateIndex("idx_memories_user_character", "memories", []string{"user_id", "character_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_memory_candidates_user", "memory_candidates", []string{"user_id", "created_at"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_retrieval_logs_user_created", "retrieval_logs", []string{"user_id", "created_at"}, false); err != nil {
				return err
			}
			return s.CreateIndex("idx_episodic_user_character", "episodic_memories", []string{"user_id", "character_id"}, false)
		},
	}
}
