package migration

func MessagePlanMigration() Migration {
	return Migration{
		Version: "202607180002",
		Name:    "add_delivery_message_plan",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS delivery_intents (
id TEXT PRIMARY KEY,
interaction_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
peer_id TEXT NOT NULL DEFAULT '',
content_type TEXT NOT NULL DEFAULT '',
payload BLOB,
status TEXT NOT NULL DEFAULT 'pending',
created_at TEXT NOT NULL DEFAULT '',
sent_at TEXT NOT NULL DEFAULT '',
delivered_at TEXT NOT NULL DEFAULT '',
retry_count INTEGER NOT NULL DEFAULT 0,
max_retries INTEGER NOT NULL DEFAULT 5,
last_error TEXT NOT NULL DEFAULT '',
lease_owner TEXT NOT NULL DEFAULT '',
lease_token TEXT NOT NULL DEFAULT '',
lease_until TEXT NOT NULL DEFAULT '',
next_retry TEXT NOT NULL DEFAULT '',
response_group_id TEXT NOT NULL DEFAULT '',
delivery_sequence INTEGER NOT NULL DEFAULT 0
)`)
			if err := s.AddColumn("delivery_intents", "response_group_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("delivery_intents", "delivery_sequence", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			return s.CreateIndex("idx_delivery_message_plan", "delivery_intents", []string{"response_group_id", "delivery_sequence"}, false)
		},
	}
}
