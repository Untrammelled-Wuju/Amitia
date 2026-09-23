package migration

func TemporalRelationshipPresenceUniqueIndexMigration() Migration {
	return Migration{
		Version: "20260923002",
		Name:    "repair_temporal_relationship_presence_unique_index",
		Up: func(s *Step) error {
			exists, err := s.TableExists("temporal_relationship_presence_states")
			if err != nil || !exists {
				return err
			}
			s.Execute(`DELETE FROM temporal_relationship_presence_states
WHERE rowid NOT IN (
	SELECT keep_rowid
	FROM (
		SELECT rowid AS keep_rowid,
			ROW_NUMBER() OVER (
				PARTITION BY space_id, character_id
				ORDER BY updated_at_utc DESC, state_version DESC, rowid DESC
			) AS row_number
		FROM temporal_relationship_presence_states
	)
	WHERE row_number = 1
)`)
			return s.CreateIndex("idx_temporal_relationship_presence_scope", "temporal_relationship_presence_states", []string{"space_id", "character_id"}, true)
		},
	}
}
