package migration

func WorkspaceMountsRemoteMigration() Migration {
	return Migration{
		Version: "202601010001",
		Name:    "add_workspace_mounts_remote_columns",
		Up: func(s *Step) error {
			s.AddColumn("workspace_mounts", "backend_config_json", "TEXT")
			s.AddColumn("workspace_mounts", "credential_ref", "TEXT")
			return nil
		},
	}
}
