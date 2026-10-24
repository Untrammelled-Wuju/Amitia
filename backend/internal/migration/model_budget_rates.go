package migration

func ModelBudgetRatesMigration() Migration {
	return Migration{
		Version: "20261024001",
		Name:    "model_budget_rate_ceiling_columns",
		Up: func(s *Step) error {
			if err := s.AddColumn("model_configs", "budget_rate_version", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("model_configs", "budget_input_rate", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			return s.AddColumn("model_configs", "budget_output_rate", "INTEGER NOT NULL DEFAULT 0")
		},
	}
}
