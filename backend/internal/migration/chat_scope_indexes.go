package migration

func ChatScopeIndexesMigration() Migration {
	return Migration{
		Version: "202607010002",
		Name:    "add_chat_scope_indexes",
		Up: func(step *Step) error {
			step.Execute("CREATE INDEX IF NOT EXISTS idx_conversations_character_id ON conversations(character_id)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_memories_character_id ON memories(character_id)")
			return nil
		},
	}
}
