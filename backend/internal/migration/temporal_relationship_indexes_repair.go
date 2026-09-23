package migration

func TemporalRelationshipIndexesRepairMigration() Migration {
	return Migration{
		Version: "20260923003",
		Name:    "repair_temporal_relationship_indexes",
		Up: func(s *Step) error {
			if err := deduplicateTemporalRows(s, "temporal_cadence_samples", "interaction_id, sample_kind", "created_at_utc DESC, rowid DESC"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_cadence_interaction", "temporal_cadence_samples", []string{"interaction_id", "sample_kind"}, true); err != nil {
				return err
			}
			if err := deduplicateTemporalRows(s, "temporal_reunion_episodes", "idempotency_key", "updated_at_utc DESC, rowid DESC"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_reunion_idempotency", "temporal_reunion_episodes", []string{"idempotency_key"}, true); err != nil {
				return err
			}
			if err := deduplicateTemporalRows(s, "temporal_interaction_receipts", "space_id, request_id", "updated_at_utc DESC, rowid DESC"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_receipt_request", "temporal_interaction_receipts", []string{"space_id", "request_id"}, true); err != nil {
				return err
			}
			if err := deduplicateTemporalRows(s, "temporal_interaction_receipts", "interaction_id", "updated_at_utc DESC, rowid DESC"); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_receipt_interaction", "temporal_interaction_receipts", []string{"interaction_id"}, true); err != nil {
				return err
			}
			if err := deduplicateTemporalRows(s, "temporal_effect_ledger", "effect_key", "applied_at_utc DESC, rowid DESC"); err != nil {
				return err
			}
			return s.CreateIndex("idx_temporal_effect_key", "temporal_effect_ledger", []string{"effect_key"}, true)
		},
	}
}

func deduplicateTemporalRows(s *Step, table, partitionColumns, orderBy string) error {
	exists, err := s.TableExists(table)
	if err != nil || !exists {
		return err
	}
	s.Execute(`DELETE FROM ` + table + `
WHERE rowid NOT IN (
	SELECT keep_rowid
	FROM (
		SELECT rowid AS keep_rowid,
			ROW_NUMBER() OVER (
				PARTITION BY ` + partitionColumns + `
				ORDER BY ` + orderBy + `
			) AS row_number
		FROM ` + table + `
	)
	WHERE row_number = 1
)`)
	return nil
}
