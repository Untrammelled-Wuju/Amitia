// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetDevicesMigration() Migration {
	return Migration{
		Version: "202608050001",
		Name:    "add_desktop_pet_devices",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_devices (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  desktop_instance_id TEXT NOT NULL,
  platform TEXT NOT NULL DEFAULT '',
  app_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  revoked_at TEXT NOT NULL DEFAULT '',
  UNIQUE(user_id, device_id)
)`)
			if err := s.CreateIndex("idx_dpd_user_device", "desktop_pet_devices", []string{"user_id", "device_id"}, true); err != nil {
				return err
			}
			return s.CreateIndex("idx_dpd_user_status", "desktop_pet_devices", []string{"user_id", "status", "revoked_at"}, false)
		},
	}
}
