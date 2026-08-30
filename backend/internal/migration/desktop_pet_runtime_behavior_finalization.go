// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

// DesktopPetRuntimeBehaviorFinalizationMigration closes the remaining
// production schema gaps for Runtime V2 physical-state reporting and the
// CloudCore -> DeviceAgent desktop-pet behavior delivery path. It is forward
// only and safe for databases that already have some of these objects because
// Step helpers and IF NOT EXISTS statements are idempotent.
func DesktopPetRuntimeBehaviorFinalizationMigration() Migration {
	return Migration{
		Version: "202608300002",
		Name:    "finalize_desktop_pet_runtime_geometry_and_behavior_mesh",
		Up: func(s *Step) error {
			for _, column := range []struct {
				table, name, definition string
			}{
				{"desktop_pet_runtime_actual_states_v2", "position_x", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_runtime_actual_states_v2", "position_y", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_runtime_actual_states_v2", "screen_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_runtime_actual_states_v2", "window_width", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_runtime_actual_states_v2", "window_height", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_runtime_actual_states_v2", "scale", "REAL NOT NULL DEFAULT 0"},
			} {
				if err := s.AddColumn(column.table, column.name, column.definition); err != nil {
					return err
				}
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_owner_mappings (
  cloud_user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  local_owner_id TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY(cloud_user_id, device_id)
)`)
			s.CreateIndex("idx_dpom_local_owner", "desktop_pet_owner_mappings", []string{"local_owner_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_affinities (
  cloud_user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  installation_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  verified_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY(cloud_user_id, character_id)
)`)
			s.CreateIndex("idx_dpbma_device", "desktop_pet_behavior_mesh_affinities", []string{"cloud_user_id", "device_id"}, false)

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_outbox (
  event_id TEXT NOT NULL DEFAULT '',
  cloud_user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  target_device_id TEXT NOT NULL DEFAULT '',
  target_installation_id TEXT NOT NULL DEFAULT '',
  reliability TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at DATETIME NOT NULL,
  claim_expires_at DATETIME,
  expires_at DATETIME,
  last_error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  delivered_at DATETIME,
  PRIMARY KEY(cloud_user_id, event_id)
)`)
			// If a pre-release build already AutoMigrated the outbox table without
			// the installation fence, add it explicitly. On a fresh database the
			// CREATE TABLE above already contains the column and AddColumn is a no-op.
			if err := s.AddColumn("desktop_pet_behavior_mesh_outbox", "target_installation_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			s.CreateIndex("idx_dpbmo_due", "desktop_pet_behavior_mesh_outbox", []string{"status", "available_at"}, false)
			s.CreateIndex("idx_dpbmo_claim", "desktop_pet_behavior_mesh_outbox", []string{"status", "claim_expires_at"}, false)
			s.CreateIndex("idx_dpbmo_expiry", "desktop_pet_behavior_mesh_outbox", []string{"status", "expires_at"}, false)
			s.CreateIndex("idx_dpbmo_character", "desktop_pet_behavior_mesh_outbox", []string{"cloud_user_id", "character_id", "status"}, false)
			s.CreateIndex("idx_dpbmo_device", "desktop_pet_behavior_mesh_outbox", []string{"cloud_user_id", "target_device_id", "status"}, false)
			return nil
		},
	}
}
