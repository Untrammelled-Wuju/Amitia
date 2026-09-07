// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func ProactiveRulesConversationIdMigration() Migration {
	return Migration{
		Version: "20260907006",
		Name:    "add_proactive_rules_conversation_id",
		Up: func(s *Step) error {
			s.AddColumn("proactive_rules", "conversation_id", "TEXT DEFAULT ''")
			return nil
		},
	}
}
