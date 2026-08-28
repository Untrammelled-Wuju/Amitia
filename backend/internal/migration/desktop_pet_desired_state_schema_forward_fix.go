// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetDesiredStateSchemaForwardFixMigration aligns the persisted
// desktop_pet_runtime_desired_states table with the canonical RuntimeDesiredState
// model used by the installation coordinator and Runtime V2 reconciliation.
//
// The original coordinator schema was installation-scoped. Runtime V2 is
// device-scoped, so existing rows are collapsed to one authoritative row per
// (user_id, device_id), preferring the active binding and then the newest
// revision. Legacy columns are intentionally retained for downgrade/read
// compatibility; this migration only adds/backfills the canonical columns.
func DesktopPetDesiredStateSchemaForwardFixMigration() Migration {
	return Migration{
		Version: "20260828001",
		Name:    "desktop_pet_desired_state_schema_forward_fix",
		Up: func(s *Step) error {
			const table = "desktop_pet_runtime_desired_states"
			columns := []struct {
				name       string
				definition string
			}{
				{"runtime_id", "TEXT NOT NULL DEFAULT ''"},
				{"pet_id", "TEXT NOT NULL DEFAULT ''"},
				{"release_id", "TEXT NOT NULL DEFAULT ''"},
				{"settings_snapshot_json", "TEXT NOT NULL DEFAULT ''"},
				{"settings_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"desired_revision", "INTEGER NOT NULL DEFAULT 0"},
				{"desired_hash", "TEXT NOT NULL DEFAULT ''"},
				{"reason", "TEXT NOT NULL DEFAULT ''"},
				{"operation_id", "TEXT NOT NULL DEFAULT ''"},
			}
			for _, column := range columns {
				if err := s.AddColumn(table, column.name, column.definition); err != nil {
					return err
				}
			}

			// Carry the legacy release/revision authority forward. Pet identity is
			// recovered from the installation row when available.
			s.Execute(`UPDATE desktop_pet_runtime_desired_states
SET release_id = CASE
        WHEN release_id = '' THEN COALESCE(NULLIF(desired_release_id, ''), '')
        ELSE release_id
    END,
    desired_revision = CASE
        WHEN desired_revision <= 0 THEN COALESCE(revision, 0)
        ELSE desired_revision
    END,
    pet_id = CASE
        WHEN pet_id = '' THEN COALESCE((
            SELECT i.pet_id
            FROM desktop_pet_installations AS i
            WHERE i.id = desktop_pet_runtime_desired_states.installation_id
            LIMIT 1
        ), '')
        ELSE pet_id
    END
WHERE 1 = 1`)

			// Preserve the highest revision ever observed for each device before rows
			// are collapsed. Reusing an old revision after migration can collide with
			// durable commands/outbox events and causes RepositoryV2 CAS updates to
			// reject every newly allocated lower revision.
			s.Execute(`INSERT OR IGNORE INTO desktop_pet_device_desired_revision_counters
    (user_id, device_id, current_revision, updated_at)
SELECT user_id, device_id, MAX(desired_revision), COALESCE(MAX(updated_at), '')
FROM desktop_pet_runtime_desired_states
WHERE user_id <> '' AND device_id <> ''
GROUP BY user_id, device_id`)
			s.Execute(`UPDATE desktop_pet_device_desired_revision_counters
SET current_revision = MAX(
        current_revision,
        COALESCE((
            SELECT MAX(d.desired_revision)
            FROM desktop_pet_runtime_desired_states AS d
            WHERE d.user_id = desktop_pet_device_desired_revision_counters.user_id
              AND d.device_id = desktop_pet_device_desired_revision_counters.device_id
        ), current_revision)
    ),
    updated_at = CASE
        WHEN updated_at = '' THEN COALESCE((
            SELECT MAX(d.updated_at)
            FROM desktop_pet_runtime_desired_states AS d
            WHERE d.user_id = desktop_pet_device_desired_revision_counters.user_id
              AND d.device_id = desktop_pet_device_desired_revision_counters.device_id
        ), '')
        ELSE updated_at
    END
WHERE EXISTS (
    SELECT 1
    FROM desktop_pet_runtime_desired_states AS d
    WHERE d.user_id = desktop_pet_device_desired_revision_counters.user_id
      AND d.device_id = desktop_pet_device_desired_revision_counters.device_id
)`)

			// Runtime V2 has one desired authority per device. Prefer the row pointed
			// to by the active binding; otherwise keep the highest/newest revision.
			s.Execute(`DELETE FROM desktop_pet_runtime_desired_states
WHERE rowid IN (
    SELECT rowid
    FROM (
        SELECT d.rowid AS rowid,
               ROW_NUMBER() OVER (
                   PARTITION BY d.user_id, d.device_id
                   ORDER BY
                       CASE WHEN EXISTS (
                           SELECT 1
                           FROM desktop_pet_device_active_installation_bindings AS b
                           WHERE b.user_id = d.user_id
                             AND b.device_id = d.device_id
                             AND b.installation_id = d.installation_id
                       ) THEN 0 ELSE 1 END,
                       d.desired_revision DESC,
                       COALESCE(d.revision, 0) DESC,
                       d.updated_at DESC,
                       d.rowid DESC
               ) AS rn
        FROM desktop_pet_runtime_desired_states AS d
        WHERE d.user_id <> '' AND d.device_id <> ''
    ) ranked
    WHERE ranked.rn > 1
)`)

			// Pending outbox rows that no longer describe the surviving device
			// authority must never be published after restart. Keep them for audit and
			// switch-recovery evidence, but move them out of the publishable state.
			s.Execute(`UPDATE desktop_pet_runtime_desired_state_outbox
SET status = 'failed',
    last_error = 'superseded_by_desired_state_schema_forward_fix'
WHERE status = 'pending'
  AND EXISTS (
      SELECT 1
      FROM desktop_pet_runtime_desired_states AS d
      WHERE d.user_id = desktop_pet_runtime_desired_state_outbox.user_id
        AND d.device_id = desktop_pet_runtime_desired_state_outbox.device_id
        AND (
            d.installation_id <> desktop_pet_runtime_desired_state_outbox.installation_id
            OR d.desired_revision <> desktop_pet_runtime_desired_state_outbox.desired_revision
        )
  )`)

			// Partial uniqueness avoids turning legacy anonymous rows into a startup
			// blocker while enforcing the canonical key used by RepositoryV2.
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS uq_dprds_user_device
ON desktop_pet_runtime_desired_states(user_id, device_id)
WHERE user_id <> '' AND device_id <> ''`)
			s.Execute(`CREATE INDEX IF NOT EXISTS idx_dprds_runtime
ON desktop_pet_runtime_desired_states(runtime_id)`)
			s.Execute(`CREATE INDEX IF NOT EXISTS idx_dprds_desired_revision
ON desktop_pet_runtime_desired_states(user_id, device_id, desired_revision)`)
			return nil
		},
	}
}
