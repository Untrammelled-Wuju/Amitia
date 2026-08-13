package migration

func CharacterCardMigration() Migration {
	return Migration{
		Version:           "202608120001",
		Name:              "add_character_card_and_worldbook_scope",
		AcceptedChecksums: []string{
			"6b8f92120d22a9380fcc4e1cf9eab4758504460d8d801de4289d43503196529b",
			"4d0fd79342e12b04b0d3ed76a7565c16916716bae60943dab10fa27ca23cda0f",
		},
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
