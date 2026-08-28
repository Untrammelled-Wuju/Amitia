// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetSchemaFinalizationMigration closes schema/model drift that would
// otherwise make fresh databases and upgraded databases behave differently.
// Every operation is forward-only and idempotent through Step helpers or
// CREATE TABLE/INDEX IF NOT EXISTS.
func DesktopPetSchemaFinalizationMigration() Migration {
	return Migration{
		Version: "202608290005",
		Name:    "finalize_desktop_pet_schema_model_alignment",
		Up: func(s *Step) error {
			for _, column := range []struct {
				table, name, definition string
			}{
				{"desktop_pet_reference_assets", "status", "TEXT NOT NULL DEFAULT 'staging'"},
				{"desktop_pet_reference_assets", "source_bytes", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_reference_assets", "normalized_bytes", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_reference_assets", "normalizer_profile_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_reference_assets", "normalizer_profile_version", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_reference_assets", "normalizer_config_hash", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_reference_assets", "normalized_artifact_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_import_stagings", "failed_at", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_import_stagings", "failure_reason", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_action_revision_event_outbox", "sequence", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_runtime_sessions", "created_at", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_runtime_sessions", "updated_at", "TEXT NOT NULL DEFAULT ''"},
			} {
				if err := s.AddColumn(column.table, column.name, column.definition); err != nil {
					return err
				}
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_dpare_aggregate_sequence", "desktop_pet_action_revision_events", []string{"aggregate_id", "sequence"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_bridge_request_snapshots (
  journal_id TEXT PRIMARY KEY,
  request_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT ''
)`)
			return nil
		},
	}
}
