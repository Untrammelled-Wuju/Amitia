package migration

// ConversationUserOwnershipMigration separates the authenticated account owner
// from character/channel scope. Existing rows are backfilled conservatively:
// first from the newest interaction record, then from sync history. Rows that
// cannot be attributed safely remain owned by the local/default account rather
// than being exposed to an arbitrary cloud account.
func ConversationUserOwnershipMigration() Migration {
	return Migration{
		Version: "20260907001",
		Name:    "add_conversation_user_ownership",
		Up: func(s *Step) error {
			if err := s.AddColumn("conversations", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			s.Execute(`UPDATE conversations
SET user_id = COALESCE(
    NULLIF((
        SELECT ir.user_id
        FROM interaction_records ir
        WHERE ir.conversation_id = conversations.id
          AND ir.user_id IS NOT NULL
          AND TRIM(ir.user_id) != ''
          AND ir.user_id != 'default'
        ORDER BY ir.updated_at DESC, ir.created_at DESC
        LIMIT 1
    ), ''),
    NULLIF((
        SELECT sc.user_id
        FROM sync_changes sc
        WHERE sc.entity_type = 'conversation'
          AND sc.entity_id = conversations.id
          AND sc.user_id IS NOT NULL
          AND TRIM(sc.user_id) != ''
          AND sc.user_id != 'default'
        ORDER BY sc.seq DESC
        LIMIT 1
    ), ''),
    NULLIF(TRIM(user_id), ''),
    'default'
)`)
			s.Execute("UPDATE conversations SET user_id = 'default' WHERE user_id IS NULL OR TRIM(user_id) = ''")
			if err := s.CreateIndex("idx_conversations_user_updated", "conversations", []string{"user_id", "updated_at"}, false); err != nil {
				return err
			}
			return s.CreateIndex("idx_conversations_user_character", "conversations", []string{"user_id", "character_id"}, false)
		},
	}
}
