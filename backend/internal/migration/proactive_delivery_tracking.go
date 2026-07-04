package migration

func ProactiveDeliveryTrackingMigration() Migration {
	return Migration{
		Version: "202607040001",
		Name:    "proactive_delivery_tracking",
		Up: func(step *Step) error {
			cols := []struct {
				name string
				def  string
			}{
			{"interaction_id", "TEXT DEFAULT ''"},
			{"delivery_intent_id", "TEXT DEFAULT ''"},
				{"delivery_id", "TEXT DEFAULT ''"},
				{"request_id", "TEXT DEFAULT ''"},
				{"delivery_status", "TEXT DEFAULT 'PENDING'"},
				{"delivered_at", "TEXT DEFAULT ''"},
				{"error_message", "TEXT DEFAULT ''"},
			}
			for _, col := range cols {
				exists, err := step.ColumnExists("proactive_messages", col.name)
				if err != nil {
					return err
				}
				if !exists {
					if err := step.AddColumn("proactive_messages", col.name, col.def); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}
