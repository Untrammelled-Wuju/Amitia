package migration

func InteractionRecordsV2Migration() Migration {
	return Migration{
		Version: "202607030001",
		Name:    "interaction_records_v2_r06",
		Up: func(step *Step) error {
			cols := []struct {
				name string
				def  string
			}{
				{"owner_instance_id", "TEXT DEFAULT ''"},
				{"heartbeat_at", "DATETIME"},
				{"commit_token", "TEXT DEFAULT ''"},
				{"commit_owner", "TEXT DEFAULT ''"},
				{"commit_acquired_at", "DATETIME"},
				{"result_message_ids", "TEXT DEFAULT ''"},
				{"delivery_intent_ids", "TEXT DEFAULT ''"},
				{"correlation_id", "TEXT DEFAULT ''"},
				{"causation_id", "TEXT DEFAULT ''"},
			}
			for _, col := range cols {
				if err := step.AddColumn("interaction_records", col.name, col.def); err != nil {
					return err
				}
			}


			if err := step.CreateIndex("idx_interaction_conv_request_unique", "interaction_records", []string{"conversation_id", "request_id"}, true); err != nil {
				return err
			}

			if err := step.CreateIndex("idx_interaction_owner_heartbeat", "interaction_records", []string{"owner_instance_id", "heartbeat_at"}, false); err != nil {
				return err
			}

			if err := step.CreateIndex("idx_interaction_commit_token", "interaction_records", []string{"commit_token"}, false); err != nil {
				return err
			}

			return nil
		},
	}
}
