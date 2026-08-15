package artifact

import (
	"github.com/u-ai/backend/internal/migration"
)

func Migration() migration.Migration {
	return migration.Migration{
		Version: "20260815002",
		Name:    "create_artifacts_and_references_tables",
		Up: func(s *migration.Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS artifacts (
				artifact_id TEXT PRIMARY KEY,
				owner_user_id TEXT NOT NULL,
				workspace_id TEXT NOT NULL DEFAULT '',
				kind TEXT NOT NULL,
				blob_digest TEXT NOT NULL,
				size_bytes INTEGER NOT NULL,
				mime_type TEXT NOT NULL,
				filename TEXT NOT NULL DEFAULT '',
				file_extension TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL,
				source TEXT NOT NULL,
				width INTEGER NOT NULL DEFAULT 0,
				height INTEGER NOT NULL DEFAULT 0,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				revision INTEGER NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				deleted_at DATETIME
			)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS artifact_references (
				artifact_id TEXT NOT NULL,
				reference_type TEXT NOT NULL,
				reference_id TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				PRIMARY KEY(artifact_id, reference_type, reference_id)
			)`)
			s.CreateIndex("idx_artifacts_owner", "artifacts", []string{"owner_user_id"}, false)
			s.CreateIndex("idx_artifacts_blob_digest", "artifacts", []string{"blob_digest"}, false)
			s.CreateIndex("idx_artifacts_status", "artifacts", []string{"status"}, false)
			s.CreateIndex("idx_artifacts_created_at", "artifacts", []string{"created_at"}, false)
			return nil
		},
	}
}
