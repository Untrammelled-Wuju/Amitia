// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetMigrationControlMigration() Migration {
	return Migration{
		Version:            "202608040004",
		Name:               "add_desktop_pet_migration_control",
		AcceptedChecksums:  []string{"6caeceab45cdc7f139c5a64b7aa94815ca993dc830de30512ba2c4bdc7a24bd7"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_operations (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}'
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_checkpoints (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  last_primary_key TEXT NOT NULL DEFAULT '',
  processed_count INTEGER NOT NULL DEFAULT 0,
  input_hash TEXT NOT NULL DEFAULT '',
  output_hash TEXT NOT NULL DEFAULT '',
  conflict_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_conflicts (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  entity_kind TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  conflict_reason TEXT NOT NULL,
  detected_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_migration_locks (
  lock_name TEXT PRIMARY KEY,
  owner_instance_id TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  heartbeat_at TEXT NOT NULL
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_read_cutovers (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  cutover_at TEXT NOT NULL,
  verified INTEGER NOT NULL DEFAULT 0
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_write_cutovers (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  cutover_at TEXT NOT NULL,
  verified INTEGER NOT NULL DEFAULT 0
)`)
			s.CreateIndex("idx_dpmc_operation", "desktop_pet_migration_checkpoints", []string{"operation_id"}, false)
			s.CreateIndex("idx_dpmc_conflict_op", "desktop_pet_migration_conflicts", []string{"operation_id"}, false)
			s.CreateIndex("idx_dpmc_lock_expiry", "desktop_pet_migration_locks", []string{"lease_expires_at"}, false)
			s.CreateIndex("idx_dprc_operation", "desktop_pet_read_cutovers", []string{"operation_id"}, false)
			s.CreateIndex("idx_dpmc_write_op", "desktop_pet_write_cutovers", []string{"operation_id"}, false)
			s.CreateIndex("idx_dprc_operation_step_unique", "desktop_pet_read_cutovers", []string{"operation_id", "step_name"}, true)
			s.CreateIndex("idx_dpwc_operation_step_unique", "desktop_pet_write_cutovers", []string{"operation_id", "step_name"}, true)
			return nil
		},
	}
}
