package migration

func ChatScopeIndexesMigration() Migration {
	return Migration{
		Version: "202607010002",
		Name:    "chat scope indexes placeholder",
		Up: func(step *Step) error {
			return nil
		},
	}
}