package migration

func MemoryContextUseMigration() Migration {
	return Migration{
		Version: "20260904003",
		Name:    "add memory context use privacy control",
		Up: func(step *Step) error {
			return step.AddColumn("memories", "allow_context_use", "INTEGER DEFAULT 1")
		},
	}
}
