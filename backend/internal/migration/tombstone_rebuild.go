package migration

func TombstoneRebuildMigration() Migration {
	return Migration{
		Version: "202607010005",
		Name:    "tombstone rebuild placeholder",
		Up: func(step *Step) error {
			return nil
		},
	}
}