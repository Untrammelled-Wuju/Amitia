package migration

func AgentSkillCutoverStateMigration() Migration {
	return Migration{
		Version: "20261021002",
		Name:    "create_agent_skill_cutover_state",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS extension_agent_skill_cutover (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				status TEXT NOT NULL DEFAULT 'pending',
				migrated_skills INTEGER NOT NULL DEFAULT 0,
				migrated_activations INTEGER NOT NULL DEFAULT 0,
				last_error TEXT NOT NULL DEFAULT '',
				started_at TEXT NOT NULL DEFAULT '',
				completed_at TEXT NOT NULL DEFAULT ''
			)`)
			return nil
		},
	}
}
