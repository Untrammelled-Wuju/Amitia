package migration

func DeviceRuntimeSessionMigration() Migration {
	return Migration{
		Version: "20260814002",
		Name:    "add_kernel_device_runtime_sessions",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS kernel_device_runtime_sessions (
    runtime_session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    connection_generation INTEGER NOT NULL DEFAULT 1,
    runtime_version TEXT NOT NULL DEFAULT '',
    runtime_contract_version TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    capabilities_hash TEXT NOT NULL DEFAULT '',
    last_applied_state_revision INTEGER NOT NULL DEFAULT 0,
    last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
    last_event_sequence INTEGER NOT NULL DEFAULT 0,
    actual_state_hash TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_heartbeat_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL DEFAULT 0,
    closed_at INTEGER NOT NULL DEFAULT 0,
    close_reason TEXT NOT NULL DEFAULT ''
)`)
			s.CreateIndex("idx_kernel_device_runtime_sessions_identity", "kernel_device_runtime_sessions", []string{"user_id", "device_id", "runtime_id"}, false)
			s.CreateIndex("idx_kernel_device_runtime_sessions_status", "kernel_device_runtime_sessions", []string{"status"}, false)
			s.CreateIndex("idx_kernel_device_runtime_sessions_heartbeat", "kernel_device_runtime_sessions", []string{"last_heartbeat_at"}, false)
			return nil
		},
	}
}

func KernelHostRegistryRuntimeSessionColumnsMigration() Migration {
	return Migration{
		Version: "20260814003",
		Name:    "add_kernel_host_registry_runtime_session_columns",
		Up: func(s *Step) error {
			s.AddColumn("kernel_host_registry", "runtime_session_id", "TEXT NOT NULL DEFAULT ''")
			s.AddColumn("kernel_host_registry", "connection_generation", "INTEGER NOT NULL DEFAULT 0")
			s.AddColumn("kernel_host_registry", "entry_id", "TEXT NOT NULL DEFAULT ''")
			return nil
		},
	}
}
