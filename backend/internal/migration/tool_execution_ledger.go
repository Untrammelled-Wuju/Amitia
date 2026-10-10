package migration

func ToolExecutionLedgerMigration() Migration {
	return Migration{
		Version: "20261009001",
		Name:    "tool_execution_durable_attempt_evidence",
		Up: func(s *Step) error {
			for _, column := range []struct {
				table string
				name  string
			}{
				{"tool_call_intents", "attempt_id"},
				{"tool_call_intents", "turn_id"},
				{"tool_call_intents", "execution_id"},
				{"tool_call_intents", "input_hash"},
				{"tool_call_intents", "owner_instance_id"},
				{"tool_call_intents", "result_ref"},
				{"tool_call_intents", "error_class"},
				{"tool_call_intents", "started_at"},
				{"tool_call_intents", "finished_at"},
				{"tool_call_results", "attempt_id"},
				{"tool_call_results", "turn_id"},
				{"tool_call_results", "execution_id"},
				{"tool_call_results", "input_hash"},
			} {
				if err := s.AddColumn(column.table, column.name, "TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
			}
			s.CreateIndex("idx_tool_call_intents_turn_call", "tool_call_intents", []string{"turn_id", "tool_call_id"}, false)
			s.CreateIndex("idx_tool_call_intents_owner", "tool_call_intents", []string{"owner_instance_id", "status"}, false)
			s.CreateIndex("idx_tool_call_results_attempt", "tool_call_results", []string{"attempt_id"}, false)
			return nil
		},
	}
}
