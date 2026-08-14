package migration

func KernelHostRegistryEntryKindMigration() Migration {
	return Migration{
		Version: "20260814002",
		Name:    "add_kernel_host_registry_entry_kind",
		AcceptedChecksums: []string{
			"c58f895c12cbaf8558c17620d4c2d62c6c15c1b32fb2ef6f720da37475161f15",
		},
		Up: func(s *Step) error {
			s.AddColumn("kernel_host_registry", "entry_kind", "TEXT NOT NULL DEFAULT 'ui_host'")
			s.CreateIndex("idx_kernel_host_reg_device", "kernel_host_registry", []string{"user_id", "device_id"}, false)
			s.CreateIndex("idx_kernel_host_reg_runtime", "kernel_host_registry", []string{"user_id", "device_id", "runtime_id"}, false)
			s.CreateIndex("idx_kernel_host_reg_kind_state", "kernel_host_registry", []string{"entry_kind", "connection_state"}, false)
			return nil
		},
	}
}
