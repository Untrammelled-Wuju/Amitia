// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetQualityInboxFinalizationMigration aligns the quality-v2 runtime
// model with its durable inbox schema and makes non-empty evaluation
// idempotency keys unique. Empty keys remain valid for legacy/manual callers.
func DesktopPetQualityInboxFinalizationMigration() Migration {
	return Migration{
		Version: "202608290003",
		Name:    "finalize_desktop_pet_quality_inbox_idempotency",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_quality_evaluation_request_inbox", "updated_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqe_idempotency_nonempty
ON desktop_pet_quality_evaluations(idempotency_key)
WHERE idempotency_key <> ''`)
			return nil
		},
	}
}
