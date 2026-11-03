package migration

func AgentControlMigration() Migration {
	return Migration{
		Version: "20261102001",
		Name:    "agent_access_attention_control",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS agent_access_policies (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				scope_type TEXT NOT NULL,
				channel_id TEXT NOT NULL DEFAULT '',
				account_id TEXT NOT NULL DEFAULT '',
				conversation_id TEXT NOT NULL DEFAULT '',
				sender_id TEXT NOT NULL DEFAULT '',
				explicit_mode TEXT NOT NULL DEFAULT 'allow',
				inferred_mode TEXT NOT NULL DEFAULT 'observe',
				trigger_mode TEXT NOT NULL DEFAULT 'direct_or_mention',
				permission_ceiling TEXT NOT NULL DEFAULT 'request_approval',
				enabled INTEGER NOT NULL DEFAULT 1,
				expires_at DATETIME,
				revision INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_agent_access_scope", "agent_access_policies", []string{"space_id", "scope_type", "channel_id", "account_id", "conversation_id", "sender_id"}, false)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS agent_access_decisions (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				request_id TEXT NOT NULL DEFAULT '',
				channel_id TEXT NOT NULL DEFAULT '',
				account_id TEXT NOT NULL DEFAULT '',
				conversation_id TEXT NOT NULL DEFAULT '',
				sender_id TEXT NOT NULL DEFAULT '',
				source TEXT NOT NULL DEFAULT '',
				trigger_mode TEXT NOT NULL DEFAULT '',
				strength TEXT NOT NULL DEFAULT '',
				decision TEXT NOT NULL DEFAULT '',
				allowed INTEGER NOT NULL DEFAULT 0,
				observed INTEGER NOT NULL DEFAULT 0,
				policy_id TEXT NOT NULL DEFAULT '',
				policy_revision INTEGER NOT NULL DEFAULT 0,
				permission_mode TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_agent_access_decision_request", "agent_access_decisions", []string{"space_id", "request_id"}, false)
			s.CreateIndex("idx_agent_access_decision_created", "agent_access_decisions", []string{"created_at"}, false)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS agent_attention_policies (
				space_id TEXT PRIMARY KEY,
				enabled INTEGER NOT NULL DEFAULT 1,
				shadow_mode INTEGER NOT NULL DEFAULT 1,
				max_interruptions_per_day INTEGER NOT NULL DEFAULT 24,
				max_repeated_confirmations INTEGER NOT NULL DEFAULT 3,
				max_consecutive_followups INTEGER NOT NULL DEFAULT 2,
				max_unchanged_notifications INTEGER NOT NULL DEFAULT 1,
				dedupe_window_minutes INTEGER NOT NULL DEFAULT 30,
				revision INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS agent_attention_events (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				role_id TEXT NOT NULL DEFAULT '',
				conversation_id TEXT NOT NULL DEFAULT '',
				source_kind TEXT NOT NULL DEFAULT '',
				source_id TEXT NOT NULL DEFAULT '',
				event_kind TEXT NOT NULL DEFAULT '',
				delivery_state TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				payload_json TEXT NOT NULL DEFAULT '{}',
				idempotency_key TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL
			)`)
			s.CreateIndex("idx_agent_attention_event_scope", "agent_attention_events", []string{"space_id", "created_at"}, false)
			s.CreateIndex("idx_agent_attention_event_key", "agent_attention_events", []string{"space_id", "idempotency_key", "created_at"}, false)
			return nil
		},
	}
}
