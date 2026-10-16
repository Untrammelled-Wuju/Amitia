package migration

func RetireLegacyReminderDataMigration() Migration {
	return Migration{Version: "20261014001", Name: "discard_retired_reminder_data", Up: func(s *Step) error {
		for _, table := range []string{"schedules", "reminders", "trigger_histories"} {
			exists, err := s.TableExists(table)
			if err != nil {
				return err
			}
			if exists {
				s.Execute("DELETE FROM " + table)
			}
		}
		if exists, err := s.TableExists("extension_owned_resources"); err != nil {
			return err
		} else if exists {
			s.Execute("DELETE FROM extension_owned_resources WHERE resource_type='schedule_create'")
		}
		return nil
	}}
}
