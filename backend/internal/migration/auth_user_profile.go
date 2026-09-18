// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func AuthUserProfileMigration() Migration {
	return Migration{
		Version: "20260904001",
		Name:    "add_auth_user_profile_fields",
		Up: func(s *Step) error {
			if err := s.AddColumn("auth_users", "nickname", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("auth_users", "user_label", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("auth_users", "bio", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return nil
		},
	}
}
