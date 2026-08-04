// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetImportStagingsMigration() Migration {
	return Migration{
		Version: "202608040003",
		Name:    "add_desktop_pet_import_stagings",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_import_stagings (
  id TEXT PRIMARY KEY,
  owner_user_id TEXT NOT NULL,
  source_filename TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  source_content_hash TEXT NOT NULL DEFAULT '',
  source_bytes INTEGER NOT NULL DEFAULT 0,
  root_kind TEXT NOT NULL DEFAULT '',
  storage_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  quarantine_path TEXT NOT NULL DEFAULT '',
  inventory_hash TEXT NOT NULL DEFAULT '',
  inventory_json TEXT NOT NULL DEFAULT '[]',
  state_revision INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumption_started_at TEXT NOT NULL DEFAULT '',
  consumed_at TEXT NOT NULL DEFAULT '',
  rejected_reason TEXT NOT NULL DEFAULT '',
  correlation_id TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex(
				"idx_dpis_owner_status",
				"desktop_pet_import_stagings",
				[]string{"owner_user_id", "status"},
				false,
			)
			return nil
		},
	}
}
