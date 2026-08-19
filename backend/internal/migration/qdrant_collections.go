package migration

func QdrantCollectionsMigration() Migration {
	return Migration{
		Version:           "qdrant:001",
		Name:              "qdrant_collections_baseline",
		AcceptedChecksums: []string{"b11e2fcb9bc79fa50f5fd2766edf8cd8cae3a61432dfb6dd069ea3c8661b4a57"},
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS qdrant_collection_versions (
				collection_name TEXT PRIMARY KEY,
				vector_dim INTEGER NOT NULL,
				distance TEXT NOT NULL DEFAULT 'Cosine',
				created_at TEXT DEFAULT ''
			)`)
			return nil
		},
	}
}
