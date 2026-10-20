package migration

// KnowledgeRevisionLifecycleMigration adds the retention/tombstone fields used
// by blue/green source revision rollback and deterministic garbage collection.
// Existing inactive revisions receive a seven-day rollback window; active
// revisions remain unbounded until a later cutover archives them.
func KnowledgeRevisionLifecycleMigration() Migration {
	return Migration{
		Version: "20260926003",
		Name:    "knowledge_revision_rollback_retention",
		Up: func(s *Step) error {
			if err := s.AddColumn("knowledge_source_revisions", "retain_until", "DATETIME"); err != nil {
				return err
			}
			if err := s.AddColumn("knowledge_source_revisions", "purged_at", "DATETIME"); err != nil {
				return err
			}
			s.CreateIndex("idx_knowledge_revisions_retention", "knowledge_source_revisions", []string{"tenant_id", "retain_until", "purged_at"}, false)

			// Revision 0 was only a transitional marker used before outbox rows had
			// an explicit source revision. Canonicalize it once during migration so
			// runtime paths never need a permanent revision=0 fallback.
			s.Execute(`UPDATE knowledge_index_outbox
				SET source_revision = (
					SELECT ks.revision FROM knowledge_sources ks
					WHERE ks.tenant_id = knowledge_index_outbox.tenant_id
					  AND ks.id = knowledge_index_outbox.aggregate_id
				)
				WHERE aggregate_type = 'knowledge_source'
				  AND source_revision = 0
				  AND EXISTS (
					SELECT 1 FROM knowledge_sources ks
					WHERE ks.tenant_id = knowledge_index_outbox.tenant_id
					  AND ks.id = knowledge_index_outbox.aggregate_id
				  )`)
			s.Execute(`DELETE FROM knowledge_index_outbox
				WHERE aggregate_type = 'knowledge_source'
				  AND source_revision = 0`)

			// Older builds could already contain inactive revisions. Give them a
			// rollback grace period instead of making them immediately GC-eligible.
			s.Execute(`UPDATE knowledge_source_revisions
				SET retain_until = datetime(COALESCE(activated_at, updated_at), '+7 days')
				WHERE purged_at IS NULL
				  AND NOT EXISTS (
					SELECT 1 FROM knowledge_sources ks
					WHERE ks.tenant_id = knowledge_source_revisions.tenant_id
					  AND ks.id = knowledge_source_revisions.source_id
					  AND ks.revision = knowledge_source_revisions.revision
				  )`)
			return nil
		},
	}
}
