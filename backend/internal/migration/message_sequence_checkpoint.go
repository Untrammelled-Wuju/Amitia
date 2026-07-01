package migration

func MessageSequenceCheckpointMigration() Migration {
	return Migration{
		Version: "202607010004",
		Name:    "message sequence checkpoint placeholder",
		Up: func(step *Step) error {
			return nil
		},
	}
}