package migration

func BackupMigration() Migration {
	return Migration{
		Version: "202607010001",
		Name:    "backup placeholder",
		Up: func(step *Step) error {
			return nil
		},
	}
}