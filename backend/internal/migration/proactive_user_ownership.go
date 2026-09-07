// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ProactiveUserOwnershipMigration() Migration {
	return Migration{
		Version: "20260907004",
		Name:    "add_proactive_user_ownership",
		Up: func(s *Step) error {
			db := s.DB()
			if err := s.AddColumn("proactive_rules", "conversation_id", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			for _, table := range []string{"proactive_rules", "proactive_messages", "reminders", "trigger_histories"} {
				if !db.Migrator().HasColumn(table, "user_id") {
					if err := db.Exec("ALTER TABLE " + table + " ADD COLUMN user_id TEXT NOT NULL DEFAULT 'default'").Error; err != nil {
						return err
					}
				}
			}
			// Conversation ownership is the canonical signal for existing reminders/rules/messages.
			for _, stmt := range []string{
				`UPDATE reminders SET user_id = COALESCE((SELECT c.user_id FROM conversations c WHERE c.id = reminders.conversation_id LIMIT 1), (SELECT ch.user_id FROM characters ch WHERE ch.id = reminders.character_id LIMIT 1), NULLIF(TRIM(user_id), ''), 'default') WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`,
				`UPDATE proactive_rules SET user_id = COALESCE((SELECT c.user_id FROM conversations c WHERE c.id = proactive_rules.conversation_id LIMIT 1), (SELECT ch.user_id FROM characters ch WHERE ch.id = proactive_rules.character_id LIMIT 1), NULLIF(TRIM(user_id), ''), 'default') WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`,
				`UPDATE proactive_messages SET user_id = COALESCE((SELECT c.user_id FROM conversations c WHERE c.id = proactive_messages.conversation_id LIMIT 1), (SELECT pr.user_id FROM proactive_rules pr WHERE pr.id = proactive_messages.rule_id LIMIT 1), NULLIF(TRIM(user_id), ''), 'default') WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`,
				`UPDATE trigger_histories SET user_id = COALESCE(CASE WHEN trigger_type = 'reminder' THEN (SELECT r.user_id FROM reminders r WHERE CAST(r.id AS TEXT) = trigger_histories.trigger_id LIMIT 1) ELSE (SELECT pr.user_id FROM proactive_rules pr WHERE CAST(pr.id AS TEXT) = trigger_histories.trigger_id LIMIT 1) END, NULLIF(TRIM(user_id), ''), 'default') WHERE TRIM(COALESCE(user_id, '')) = '' OR user_id = 'default'`,
			} {
				s.Execute(stmt)
			}
			for _, stmt := range []string{
				"CREATE INDEX IF NOT EXISTS idx_proactive_rules_user ON proactive_rules(user_id, character_id, enabled)",
				"CREATE INDEX IF NOT EXISTS idx_reminders_user ON reminders(user_id, enabled, remind_at)",
				"CREATE INDEX IF NOT EXISTS idx_proactive_messages_user ON proactive_messages(user_id, created_at)",
				"CREATE INDEX IF NOT EXISTS idx_trigger_histories_user ON trigger_histories(user_id, created_at)",
			} {
				s.Execute(stmt)
			}
			return nil
		},
	}
}
