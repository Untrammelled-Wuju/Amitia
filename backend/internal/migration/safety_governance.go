package migration

func SafetyGovernanceMigration() Migration {
	return Migration{
		Version: "20261023006",
		Name:    "create_scoped_safety_governance",
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS safety_policies (
				space_id TEXT NOT NULL,
				owner_id TEXT NOT NULL,
				core_id TEXT NOT NULL,
				revision INTEGER NOT NULL DEFAULT 1,
				config_json TEXT NOT NULL DEFAULT '{}',
				bdi_json TEXT NOT NULL DEFAULT '{}',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				PRIMARY KEY(space_id, owner_id)
			)`)
			step.CreateTable(`CREATE TABLE IF NOT EXISTS safety_incidents (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				owner_id TEXT NOT NULL,
				core_id TEXT NOT NULL,
				role_id TEXT NOT NULL DEFAULT '',
				conversation_id TEXT NOT NULL DEFAULT '',
				message_id TEXT NOT NULL DEFAULT '',
				feedback_id INTEGER,
				request_id TEXT NOT NULL DEFAULT '',
				event_type TEXT NOT NULL,
				severity TEXT NOT NULL DEFAULT 'medium',
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'open',
				policy_revision INTEGER NOT NULL DEFAULT 0,
				revision INTEGER NOT NULL DEFAULT 1,
				details_json TEXT NOT NULL DEFAULT '{}',
				legacy_event_id TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT '',
				resolved_at TEXT NOT NULL DEFAULT '',
				resolved_by TEXT NOT NULL DEFAULT ''
			)`)
			step.CreateIndex("idx_safety_policies_owner", "safety_policies", []string{"space_id", "owner_id", "core_id"}, false)
			step.CreateIndex("idx_safety_incidents_scope", "safety_incidents", []string{"space_id", "owner_id", "core_id", "status", "created_at"}, false)
			step.CreateIndex("idx_safety_incidents_conversation", "safety_incidents", []string{"conversation_id"}, false)
			step.CreateIndex("idx_safety_incidents_message", "safety_incidents", []string{"message_id"}, false)
			step.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_safety_incidents_feedback ON safety_incidents(feedback_id) WHERE feedback_id IS NOT NULL")
			step.Execute("CREATE UNIQUE INDEX IF NOT EXISTS uq_safety_incidents_legacy ON safety_incidents(legacy_event_id) WHERE legacy_event_id <> ''")
			return nil
		},
	}
}
