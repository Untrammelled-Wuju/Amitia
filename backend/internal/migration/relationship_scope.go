package migration

func RelationshipScopeMigration() Migration {
	return Migration{
		Version: "017",
		Name:    "add_relationship_user_scope",
		Up: func(s *Step) error {
			if err := s.AddColumn("relationship_states", "user_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
				return err
			}
			if err := s.AddColumn("relationship_states", "channel", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return s.CreateIndex("idx_relationship_user_char", "relationship_states", []string{"user_id", "character_id", "relation_type"}, false)
		},
	}
}
