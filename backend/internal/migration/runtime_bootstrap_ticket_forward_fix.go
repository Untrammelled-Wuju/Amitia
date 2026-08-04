package migration

func RuntimeBootstrapTicketRuntimeIDForwardFix() Migration {
	return Migration{
		Version: "202608040001",
		Name:    "add_runtime_id_to_runtime_bootstrap_tickets",
		Up: func(s *Step) error {
			if err := s.AddColumn(
				"desktop_pet_runtime_bootstrap_tickets",
				"runtime_id",
				"TEXT NOT NULL DEFAULT ''",
			); err != nil {
				return err
			}

			if err := s.CreateIndex(
				"idx_dprbt_runtime",
				"desktop_pet_runtime_bootstrap_tickets",
				[]string{"runtime_id"},
				false,
			); err != nil {
				return err
			}

			return nil
		},
	}
}
