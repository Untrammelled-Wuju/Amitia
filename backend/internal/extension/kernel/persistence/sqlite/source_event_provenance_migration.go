package sqlite

func init() {
	schemaMigrations = append(schemaMigrations, `CREATE TABLE IF NOT EXISTS extension_event_host_provenance (
		outbox_id TEXT PRIMARY KEY,
		provenance_json TEXT NOT NULL,
		FOREIGN KEY(outbox_id) REFERENCES extension_event_outbox(outbox_id) ON DELETE CASCADE
	)`)
}
