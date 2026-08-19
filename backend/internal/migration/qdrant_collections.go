package migration

func QdrantCollectionsMigration() Migration {
	return Migration{
		Version: "qdrant:001",
		Name:    "qdrant_collections_baseline",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS qdrant_collection_versions (
				collection_name TEXT PRIMARY KEY,
				vector_dim INTEGER NOT NULL,
				distance TEXT NOT NULL DEFAULT 'Cosine',
				created_at TEXT DEFAULT (datetime('now'))
			)`)
			return nil
		},
	}
}
