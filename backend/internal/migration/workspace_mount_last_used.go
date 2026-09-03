package migration

func WorkspaceMountLastUsedMigration() Migration {
	return Migration{
		Version: "20260903003",
		Name:    "add_workspace_mount_last_used",
		Up: func(s *Step) error {
			if err := s.AddColumn("workspace_mounts", "last_used_at", "TEXT DEFAULT ''"); err != nil {
				return err
			}
			s.Execute("UPDATE workspace_mounts SET last_used_at = updated_at WHERE COALESCE(last_used_at, '') = ''")
			return nil
		},
	}
}
