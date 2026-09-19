package migration

func SidebarPreferencesMigration() Migration {
	return Migration{
		Version: "20260919002",
		Name:    "add_sidebar_pin_and_archive_state",
		Up: func(s *Step) error {
			if err := s.AddColumn("conversations", "pinned_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("conversations", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("projects", "pinned_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			s.CreateIndex("idx_conversations_sidebar_pinned", "conversations", []string{"space_id", "pinned_at", "updated_at"}, false)
			s.CreateIndex("idx_conversations_sidebar_archived", "conversations", []string{"space_id", "archived_at", "updated_at"}, false)
			s.CreateIndex("idx_projects_sidebar_pinned", "projects", []string{"space_id", "pinned_at", "updated_at"}, false)
			return nil
		},
	}
}
