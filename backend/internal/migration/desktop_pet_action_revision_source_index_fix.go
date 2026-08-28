// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetActionRevisionSourceIndexFixMigration narrows the historical
// (source_processing_revision_id, source_type) uniqueness rule to the only
// source type that is intrinsically one-to-one with a ProcessingRevision:
// processing_baseline. Manual edits and full-action regenerations are normal
// multi-version descendants of the same processing baseline and therefore
// must not be globally unique by source_processing_revision_id.
func DesktopPetActionRevisionSourceIndexFixMigration() Migration {
	return Migration{
		Version: "202608290001",
		Name:    "desktop_pet_action_revision_source_index_fix",
		Up: func(s *Step) error {
			s.Execute("DROP INDEX IF EXISTS uq_dpar_source_type")
			s.Execute("DROP INDEX IF EXISTS idx_dpar_source_type")
			s.Execute("DROP INDEX IF EXISTS uq_dpar_processing_baseline_source")
			s.Execute(`CREATE INDEX IF NOT EXISTS idx_dpar_source_type
ON desktop_pet_action_revisions(source_processing_revision_id, source_type)`)
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS uq_dpar_processing_baseline_source
ON desktop_pet_action_revisions(source_processing_revision_id)
WHERE source_type = 'processing_baseline' AND source_processing_revision_id <> ''`)
			return nil
		},
	}
}
