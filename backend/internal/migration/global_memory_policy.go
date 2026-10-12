package migration

func GlobalMemoryPolicyMigration() Migration {
	return Migration{Version: "20261012001", Name: "global_memory_policy", Up: func(step *Step) error {
		if err := step.AddColumn("characters", "global_memory_enabled", "INTEGER NOT NULL DEFAULT 1"); err != nil {
			return err
		}
		step.CreateTable("CREATE TABLE IF NOT EXISTS memory_scope_transfers (owner_id TEXT NOT NULL,request_id TEXT NOT NULL,payload_hash TEXT NOT NULL,ack TEXT NOT NULL,record_kind TEXT NOT NULL,record_id TEXT NOT NULL,original_record TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(owner_id,request_id))")
		return nil
	}}
}
