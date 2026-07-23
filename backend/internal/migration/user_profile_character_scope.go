package migration

func UserProfileCharacterScopeMigration() Migration {
	return Migration{
		Version: "202607230001",
		Name:    "add_user_profile_character_scope",
		Up: func(step *Step) error {
			if err := step.AddColumn("user_profiles", "character_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := step.AddColumn("user_profiles", "source", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			step.Execute("DROP INDEX IF EXISTS idx_user_profiles_uid_cat_attr")
			step.Execute("CREATE UNIQUE INDEX IF NOT EXISTS idx_user_profiles_uid_cat_attr ON user_profiles(user_id, character_id, category, attribute_name)")
			return nil
		},
	}
}
