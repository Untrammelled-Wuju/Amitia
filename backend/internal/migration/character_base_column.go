package migration

func CharacterBaseColumnMigration() Migration {
	return Migration{
		Version: "082",
		Name:    "add_character_base_column",
		Up: func(s *Step) error {
			if err := s.AddColumn("characters", "character_base", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			tableExists, err := s.TableExists("characters")
			if err != nil {
				return err
			}
			if tableExists {
				s.Execute("UPDATE characters SET character_base = system_prompt WHERE character_base = '' AND system_prompt != ''")
			}
			return nil
		},
	}
}
