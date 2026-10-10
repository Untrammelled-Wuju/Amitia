package migration

func ConversationReadStateMigration() Migration {
	return Migration{
		Version: "20261009002",
		Name:    "add_conversation_read_state",
		Up: func(s *Step) error {
			if err := s.AddColumn("conversations", "last_read_turn_sequence", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			s.Execute(`UPDATE conversations SET last_read_turn_sequence = COALESCE((SELECT MAX(sequence) FROM assistant_turns WHERE assistant_turns.conversation_id = conversations.id AND assistant_turns.status IN ('completed','failed','interrupted')), 0)`)
			return nil
		},
	}
}
