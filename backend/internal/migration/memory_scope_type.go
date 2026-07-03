package migration

func MemoryScopeTypeMigration() Migration {
	return Migration{
		Version: "202607010102",
		Name:    "add memory scope type",
		Up: func(step *Step) error {
			if err := step.AddColumn("memories", "scope", "TEXT DEFAULT 'character'"); err != nil {
				return err
			}
			if err := step.AddColumn("memories", "scope_type", "TEXT DEFAULT 'user_character'"); err != nil {
				return err
			}
			step.Execute(`UPDATE memories
SET scope_type = CASE
    WHEN scope_type IN ('character_self', 'world') THEN scope_type
    WHEN scope = 'user' THEN 'user_global'
    WHEN scope_type = 'user_global' THEN 'user_global'
    WHEN scope = 'character' THEN 'user_character'
    ELSE 'user_character'
END
WHERE scope_type IS NULL OR scope_type = '' OR scope_type IN ('user', 'character', 'user_character')`)
			return step.CreateIndex("idx_memories_scope_type", "memories", []string{"scope_type"}, false)
		},
	}
}
