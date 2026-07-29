package migration

func PipelineCheckpointLocalTimeMigration() Migration {
	return Migration{
		Version:           "202607260002",
		Name:              "convert_pipeline_checkpoint_times_to_local",
		AcceptedChecksums: []string{"945a99577a235cd6c38ba8c5a55e52b27c221df35931606b31772041c768accc"},
		Up: func(step *Step) error {
			exists, err := step.TableExists("pipeline_checkpoints")
			if err != nil || !exists {
				return err
			}
			hasLeaseExpires, err := step.ColumnExists("pipeline_checkpoints", "lease_expires_at")
			if err != nil {
				return err
			}
			if hasLeaseExpires {
				step.Execute("UPDATE pipeline_checkpoints SET created_at = CASE WHEN created_at != '' THEN datetime(created_at, 'localtime') ELSE created_at END, updated_at = CASE WHEN updated_at != '' THEN datetime(updated_at, 'localtime') ELSE updated_at END, lease_expires_at = CASE WHEN lease_expires_at != '' THEN datetime(lease_expires_at, 'localtime') ELSE lease_expires_at END")
			} else {
				step.Execute("UPDATE pipeline_checkpoints SET created_at = CASE WHEN created_at != '' THEN datetime(created_at, 'localtime') ELSE created_at END, updated_at = CASE WHEN updated_at != '' THEN datetime(updated_at, 'localtime') ELSE updated_at END")
			}
			return nil
		},
	}
}
