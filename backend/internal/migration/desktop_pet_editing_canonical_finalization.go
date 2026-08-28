// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetEditingCanonicalFinalizationMigration aligns candidate metadata
// with the canonical ActionStream lineage used by regeneration acceptance.
// Existing databases need the same parent content hash column that fresh
// baseline installs already expose.
func DesktopPetEditingCanonicalFinalizationMigration() Migration {
	return Migration{
		Version: "202608290004",
		Name:    "finalize_desktop_pet_editing_canonical_lineage",
		Up: func(s *Step) error {
			return s.AddColumn("desktop_pet_candidate_revision_metadata", "parent_content_hash", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
