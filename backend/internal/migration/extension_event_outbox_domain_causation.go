package migration

func ExtensionEventOutboxDomainCausationMigration() Migration {
	return Migration{
		Version: "202608140001",
		Name:    "add_extension_event_outbox_domain_causation",
		Up: func(s *Step) error {
			if err := s.AddColumn("extension_event_outbox", "event_domain", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return s.AddColumn("extension_event_outbox", "causation_id", "TEXT NOT NULL DEFAULT ''")
		},
	}
}
