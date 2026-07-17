package migration

func ExtensionScheduleSourceMigration() Migration {
	return Migration{
		Version: "202607170014",
		Name:    "add_extension_schedule_source_type",
		Up: func(s *Step) error {
			exists, err := s.TableExists("schedules")
			if err != nil {
				return err
			}
			if err := s.AddColumn("schedules", "source_type", "TEXT NOT NULL DEFAULT 'user'"); err != nil {
				return err
			}
			statement := "UPDATE schedules SET source_type = 'extension' WHERE source_extension_id <> ''"
			if exists {
				s.Execute(statement)
			} else {
				s.operations = append(s.operations, statement)
			}
			return nil
		},
	}
}
