// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetBehaviorInboxTenantDedupMigration replaces the pre-release global
// dedup fence with the canonical per-user/per-character fence. Different
// tenants are allowed to produce the same semantic lifecycle dedup key.
func DesktopPetBehaviorInboxTenantDedupMigration() Migration {
	return Migration{
		Version: "202608310002",
		Name:    "finalize_desktop_pet_behavior_inbox_tenant_dedup",
		Up: func(s *Step) error {
			s.Execute("DROP INDEX IF EXISTS ux_desktop_pet_behavior_inbox_dedup")
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS ux_desktop_pet_behavior_inbox_dedup
ON desktop_pet_behavior_inbox(user_id, character_id, dedup_key)`)
			return nil
		},
	}
}
