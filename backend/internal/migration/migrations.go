package migration

func DefaultMigrations() []Migration {
	return []Migration{
		BackupMigration(),
		{
			Version: "001",
			Name:    "add_psyche_states_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_states (character_id TEXT PRIMARY KEY, version TEXT DEFAULT '', state_version INTEGER DEFAULT 0, emotion TEXT DEFAULT '{}', mood TEXT DEFAULT '{}', stress REAL DEFAULT 0, energy REAL DEFAULT 0.7, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "002",
			Name:    "add_psyche_events_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_events (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL DEFAULT '', event_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "003",
			Name:    "add_psyche_snapshots_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS psyche_snapshots (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', snapshot_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "004",
			Name:    "add_relationship_states_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS relationship_states (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', relation_type TEXT NOT NULL DEFAULT '', relation_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')")
				return nil
			},
		},
		{
			Version: "005",
			Name:    "add_relationship_events_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS relationship_events (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', event_type TEXT NOT NULL DEFAULT '', event_data TEXT DEFAULT '{}', created_at TEXT DEFAULT '')")
				return nil
			},
		},
		MemoryScopeTypeMigration(),
		MemorySensitivityMigration(),
		ChatScopeIndexesMigration(),
		MessageSequenceCheckpointMigration(),
		TombstoneRebuildMigration(),
		InteractionRecordsCreateMigration(),
		InteractionRecordsV2Migration(),
		ProactiveDeliveryTrackingMigration(),
		RuntimeQueueMigration(),
		LegacyDataMigration(),
		TriggerHistoryMigration(),
		RelationshipScopeMigration(),
		NeedStatesMigration(),
	}
}
