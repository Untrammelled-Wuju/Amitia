package migration

func QdrantOwnedProjectionMigration() Migration {
	return Migration{Version: "qdrant:002", Name: "device_owned_vector_projection_contract", Up: func(s *Step) error {
		return s.AddColumn("qdrant_collection_versions", "schema_version", "TEXT NOT NULL DEFAULT ''")
	}}
}

func SurrealOwnedProjectionMigration() Migration {
	return Migration{Version: "surreal:002", Name: "device_owned_graph_projection_contract", Up: func(s *Step) error {
		return s.AddColumn("surreal_schema_versions", "projection_contract", "TEXT NOT NULL DEFAULT ''")
	}}
}
