package migration

func LegacyDataMigration() Migration {
	return Migration{
		Version: "202607010003",
		Name:    "legacy data migration placeholder",
		Up: func(step *Step) error {
			return nil
		},
	}
}