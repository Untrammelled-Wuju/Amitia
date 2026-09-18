package migration

func MessageFeedbackColumnsMigration() Migration {
	return Migration{
		Version: "20260914002",
		Name:    "add_message_feedback_columns",
		Up: func(step *Step) error {
			if err := step.AddColumn("message_feedback", "feedback_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			return step.AddColumn("message_feedback", "reason", "TEXT DEFAULT ''")
		},
	}
}
