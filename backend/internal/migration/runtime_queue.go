package migration

func RuntimeQueueMigration() Migration {
	return Migration{
		Version: "202607030002",
		Name:    "runtime_queue_r12",
		Up: func(step *Step) error {
			step.CreateTable(`CREATE TABLE IF NOT EXISTS runtime_queue (
				task_id TEXT PRIMARY KEY,
				scope TEXT NOT NULL DEFAULT '',
				priority INTEGER NOT NULL DEFAULT 5,
				status TEXT NOT NULL DEFAULT 'pending',
				available_at DATETIME,
				deadline DATETIME,
				lease TEXT DEFAULT '',
				attempt INTEGER DEFAULT 0,
				payload_version INTEGER DEFAULT 1
			)`)

			if err := step.CreateIndex("idx_runtime_queue_priority_available", "runtime_queue", []string{"priority", "available_at"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_runtime_queue_scope", "runtime_queue", []string{"scope"}, false); err != nil {
				return err
			}
			if err := step.CreateIndex("idx_runtime_queue_lease", "runtime_queue", []string{"lease"}, false); err != nil {
				return err
			}

			return nil
		},
	}
}
