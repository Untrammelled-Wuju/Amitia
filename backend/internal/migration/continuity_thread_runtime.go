package migration

func ContinuityThreadRuntimeMigration() Migration {
	return Migration{
		Version: "20260922001",
		Name:    "continuity_thread_runtime",
		Up: func(s *Step) error {
			if err := s.AddColumn("interaction_records", "thread_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_interaction_records_thread_id", "interaction_records", []string{"thread_id"}, false); err != nil {
				return err
			}
			s.CreateTable(`CREATE TABLE IF NOT EXISTS continuity_threads (
				id TEXT PRIMARY KEY,
				space_id TEXT NOT NULL,
				character_id TEXT NOT NULL DEFAULT '',
				parent_thread_id TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL,
				goal TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'active',
				summary TEXT NOT NULL DEFAULT '',
				current_state TEXT NOT NULL DEFAULT '',
				next_action TEXT NOT NULL DEFAULT '',
				priority INTEGER NOT NULL DEFAULT 0,
				confidence REAL NOT NULL DEFAULT 1,
				revision INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				last_active_at DATETIME NOT NULL,
				completed_at DATETIME
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS continuity_thread_bindings (
				id TEXT PRIMARY KEY,
				thread_id TEXT NOT NULL,
				binding_type TEXT NOT NULL,
				binding_id TEXT NOT NULL,
				role TEXT NOT NULL DEFAULT 'context',
				confidence REAL NOT NULL DEFAULT 1,
				source TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL,
				last_active_at DATETIME NOT NULL,
				UNIQUE(thread_id, binding_type, binding_id)
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS continuity_thread_events (
				id TEXT PRIMARY KEY,
				thread_id TEXT NOT NULL,
				event_type TEXT NOT NULL,
				source_type TEXT NOT NULL DEFAULT '',
				source_id TEXT NOT NULL DEFAULT '',
				conversation_id TEXT NOT NULL DEFAULT '',
				request_id TEXT NOT NULL DEFAULT '',
				execution_id TEXT NOT NULL DEFAULT '',
				payload_json TEXT NOT NULL DEFAULT '{}',
				idempotency_key TEXT NOT NULL UNIQUE,
				occurred_at DATETIME NOT NULL
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS continuity_waits (
				id TEXT PRIMARY KEY,
				thread_id TEXT NOT NULL,
				wait_type TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'waiting',
				description TEXT NOT NULL DEFAULT '',
				condition_json TEXT NOT NULL DEFAULT '{}',
				resume_hint TEXT NOT NULL DEFAULT '',
				due_at DATETIME,
				resolved_at DATETIME,
				source_execution_id TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`)
			for _, idx := range []struct {
				name  string
				table string
				cols  []string
			}{
				{"idx_continuity_threads_scope_status", "continuity_threads", []string{"space_id", "status", "last_active_at"}},
				{"idx_continuity_threads_character", "continuity_threads", []string{"character_id", "last_active_at"}},
				{"idx_continuity_bindings_lookup", "continuity_thread_bindings", []string{"binding_type", "binding_id", "last_active_at"}},
				{"idx_continuity_events_thread_time", "continuity_thread_events", []string{"thread_id", "occurred_at"}},
				{"idx_continuity_waits_thread_status", "continuity_waits", []string{"thread_id", "status", "wait_type"}},
			} {
				if err := s.CreateIndex(idx.name, idx.table, idx.cols, false); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
