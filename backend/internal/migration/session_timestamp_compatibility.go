package migration

import "strings"

func SessionTimestampCompatibilityMigration() Migration {
	return Migration{
		Version: "20260820001",
		Name:    "rebuild_session_timestamp_columns",
		Up: func(s *Step) error {
			var authExpiresType string
			if err := s.DB().Raw("SELECT type FROM pragma_table_info('auth_sessions') WHERE name = 'expires_at'").Scan(&authExpiresType).Error; err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(authExpiresType), "TEXT") {
				s.Execute(`CREATE TABLE auth_sessions_timestamp_new (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					username TEXT NOT NULL DEFAULT '',
					role TEXT NOT NULL DEFAULT 'user',
					token_hash TEXT NOT NULL,
					device_name TEXT DEFAULT '',
					ip_address TEXT DEFAULT '',
					user_agent TEXT DEFAULT '',
					last_active_at DATETIME,
					expires_at DATETIME,
					created_at DATETIME NOT NULL,
					public_id TEXT UNIQUE,
					status TEXT NOT NULL DEFAULT 'active',
					revision INTEGER NOT NULL DEFAULT 1,
					absolute_expires_at DATETIME,
					revoked_at DATETIME,
					revoke_reason TEXT,
					last_refreshed_at DATETIME
				)`)
				s.Execute(`INSERT INTO auth_sessions_timestamp_new (id,user_id,username,role,token_hash,device_name,ip_address,user_agent,last_active_at,expires_at,created_at,public_id,status,revision,absolute_expires_at,revoked_at,revoke_reason,last_refreshed_at)
					SELECT id,user_id,username,role,token_hash,device_name,ip_address,user_agent,NULLIF(last_active_at,''),NULLIF(expires_at,''),created_at,public_id,status,revision,NULLIF(absolute_expires_at,''),NULLIF(revoked_at,''),revoke_reason,NULLIF(last_refreshed_at,'') FROM auth_sessions`)
				s.Execute("DROP TABLE auth_sessions")
				s.Execute("ALTER TABLE auth_sessions_timestamp_new RENAME TO auth_sessions")
				s.Execute("CREATE INDEX IF NOT EXISTS idx_auth_sessions_public_id ON auth_sessions(public_id)")
				s.Execute("CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_status ON auth_sessions(user_id, status)")
			}

			var desktopExpiresType string
			if err := s.DB().Raw("SELECT type FROM pragma_table_info('desktop_pet_local_sessions') WHERE name = 'expires_at'").Scan(&desktopExpiresType).Error; err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(desktopExpiresType), "TEXT") {
				s.Execute(`CREATE TABLE desktop_pet_local_sessions_timestamp_new (
					id TEXT PRIMARY KEY,
					user_id TEXT NOT NULL DEFAULT '',
					desktop_instance_id TEXT NOT NULL DEFAULT '',
					token_hash TEXT NOT NULL DEFAULT '',
					status TEXT NOT NULL DEFAULT 'active',
					created_at DATETIME NOT NULL,
					expires_at DATETIME NOT NULL,
					last_used_at DATETIME NOT NULL,
					revoked_at DATETIME
				)`)
				s.Execute(`INSERT INTO desktop_pet_local_sessions_timestamp_new (id,user_id,desktop_instance_id,token_hash,status,created_at,expires_at,last_used_at,revoked_at)
					SELECT id,user_id,desktop_instance_id,token_hash,status,created_at,expires_at,last_used_at,NULLIF(revoked_at,'') FROM desktop_pet_local_sessions`)
				s.Execute("DROP TABLE desktop_pet_local_sessions")
				s.Execute("ALTER TABLE desktop_pet_local_sessions_timestamp_new RENAME TO desktop_pet_local_sessions")
				s.Execute("CREATE INDEX IF NOT EXISTS idx_dpls_token ON desktop_pet_local_sessions(token_hash, status)")
				s.Execute("CREATE INDEX IF NOT EXISTS idx_dpls_user ON desktop_pet_local_sessions(user_id, status)")
			}
			return nil
		},
	}
}
