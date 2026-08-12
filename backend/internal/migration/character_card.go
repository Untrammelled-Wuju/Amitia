package migration

func CharacterCardMigration() Migration {
	return Migration{
		Version: "202608120001",
		Name:    "add_character_card_and_worldbook_scope",
		Up: func(step *Step) error {
			step.AddColumn("characters", "card_data_json", "TEXT DEFAULT '{}'")
			step.AddColumn("world_book", "character_id", "TEXT DEFAULT ''")
			step.AddColumn("world_book", "config_json", "TEXT DEFAULT '{}'")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_world_book_character_id ON world_book(character_id)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_world_book_character_priority ON world_book(character_id, priority)")
			step.Execute("CREATE INDEX IF NOT EXISTS idx_conversations_character_updated ON conversations(character_id, updated_at)")
			return nil
		},
	}
}
