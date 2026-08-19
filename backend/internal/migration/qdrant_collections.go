package migration

func QdrantCollectionsMigration() Migration {
	return Migration{
		Version:           "qdrant:001",
		Name:              "qdrant_collections_baseline",
		AcceptedChecksums: []string{
			"b11e2fcb9bc79fa50f5fd2766edf8cd8cae3a61432dfb6dd069ea3c8661b4a57",
			"78ee3607dd608cf6801f95ccaad984c68f3061245a81e132a8ba97156c90082b",
		},
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
