package migration

func RuntimeBootstrapTicketMigration() Migration {
	return Migration{
		Version: "202608030002",
		Name:    "add_runtime_bootstrap_tickets",
		Up: func(s *Step) error {
			s.Execute(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_bootstrap_tickets (
				id TEXT PRIMARY KEY,
				ticket_hash TEXT UNIQUE NOT NULL,
				user_id TEXT NOT NULL DEFAULT '',
				device_id TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				expires_at TEXT NOT NULL DEFAULT '',
				consumed_at TEXT NOT NULL DEFAULT '',
				consumed_by_runtime TEXT NOT NULL DEFAULT '',
				reason TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL DEFAULT ''
			)`)
			s.CreateIndex("idx_bt_user", "desktop_pet_runtime_bootstrap_tickets", []string{"user_id"}, false)
			s.CreateIndex("idx_bt_device", "desktop_pet_runtime_bootstrap_tickets", []string{"device_id"}, false)
			s.CreateIndex("idx_bt_status", "desktop_pet_runtime_bootstrap_tickets", []string{"status"}, false)
			return nil
		},
	}
}
