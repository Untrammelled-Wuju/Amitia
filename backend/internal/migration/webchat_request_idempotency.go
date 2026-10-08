package migration

func WebChatRequestIdempotencyMigration() Migration {
	return Migration{
		Version: "20261007005",
		Name:    "webchat_request_idempotency",
		Up: func(s *Step) error {
			// Historical single-process deployments could theoretically contain
			// duplicate non-empty request IDs. Preserve every row by moving only
			// the later duplicates into a clearly marked legacy namespace before
			// enforcing the Cloud multi-replica uniqueness boundary.
			s.Execute(`
WITH ranked AS (
	SELECT
		id,
		ROW_NUMBER() OVER (
			PARTITION BY conversation_id, request_id
			ORDER BY created_at, sequence, id
		) AS rn
	FROM assistant_turns
	WHERE request_id <> ''
)
UPDATE assistant_turns
SET request_id = request_id || ':legacy:' || id
WHERE id IN (SELECT id FROM ranked WHERE rn > 1)
`)
			s.Execute(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_assistant_turns_conv_request_unique
ON assistant_turns(conversation_id, request_id)
WHERE request_id <> ''
`)
			return nil
		},
	}
}
