package migration

func QdrantMigration() Migration {
	return QdrantCollectionsMigration()
}

func SurrealMigration() Migration {
	return SurrealSchemaMigration()
}
