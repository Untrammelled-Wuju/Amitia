package migration

func MessageSequenceCheckpointMigration() Migration {
 	return Migration{
 		Version: "202607010004",
		Name:    "create_message_sequence_checkpoints_table",
 		Up: func(step *Step) error {
			step.CreateTable("CREATE TABLE IF NOT EXISTS message_sequence_checkpoints (id TEXT PRIMARY KEY, character_id TEXT NOT NULL DEFAULT '', conversation_id TEXT NOT NULL DEFAULT '', last_sequence INTEGER DEFAULT 0, last_message_id TEXT NOT NULL DEFAULT '', processed_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'active', updated_at TEXT DEFAULT '')")
			return nil
 		},
 	}
 }
