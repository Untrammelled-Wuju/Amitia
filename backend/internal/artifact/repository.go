package artifact

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	Create(tx *gorm.DB, artifact *Artifact) error
	CreateSqlTx(tx *sql.Tx, artifact *Artifact) error
	GetByID(id ID) (*Artifact, error)
	GetByOwnerAndID(ownerUserID string, id ID) (*Artifact, error)
	SoftDelete(tx *gorm.DB, id ID) error
	SoftDeleteSqlTx(tx *sql.Tx, id ID) error
	InsertReference(tx *gorm.DB, ref *ArtifactReference) error
	InsertReferenceSqlTx(tx *sql.Tx, ref *ArtifactReference) error
	RemoveReferenceSqlTx(tx *sql.Tx, artifactID ID, refType string, refID string) error
	CountReferences(artifactID ID) (int64, error)
	CountReferencesSqlTx(tx *sql.Tx, artifactID ID) (int64, error)
	DB() *gorm.DB
	SqlDB() (*sql.DB, error)
}

type sqliteRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &sqliteRepository{db: db}
}

func (r *sqliteRepository) DB() *gorm.DB {
	return r.db
}

func (r *sqliteRepository) SqlDB() (*sql.DB, error) {
	return r.db.DB()
}

func (r *sqliteRepository) Create(tx *gorm.DB, artifact *Artifact) error {
	return tx.Create(artifact).Error
}

func (r *sqliteRepository) CreateSqlTx(tx *sql.Tx, artifact *Artifact) error {
	_, err := tx.Exec(
		`INSERT INTO artifacts (
			artifact_id, owner_user_id, workspace_id, kind, blob_digest,
			size_bytes, mime_type, filename, file_extension, status,
			source, width, height, duration_ms, revision,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID, artifact.OwnerUserID, artifact.WorkspaceID, artifact.Kind, artifact.BlobDigest,
		artifact.SizeBytes, artifact.MIMEType, artifact.Filename, artifact.Extension, artifact.Status,
		artifact.Source, artifact.Width, artifact.Height, artifact.DurationMS, artifact.Revision,
		artifact.CreatedAt, artifact.UpdatedAt,
	)
	return err
}

func (r *sqliteRepository) GetByID(id ID) (*Artifact, error) {
	var a Artifact
	err := r.db.Where("artifact_id = ?", id).First(&a).Error
	return &a, err
}

func (r *sqliteRepository) GetByOwnerAndID(ownerUserID string, id ID) (*Artifact, error) {
	var a Artifact
	err := r.db.Where("artifact_id = ? AND owner_user_id = ?", id, ownerUserID).First(&a).Error
	return &a, err
}

func (r *sqliteRepository) SoftDelete(tx *gorm.DB, id ID) error {
	now := r.db.NowFunc()
	return tx.Model(&Artifact{}).Where("artifact_id = ?", id).Updates(map[string]interface{}{
		"status":     StatusDeleted,
		"deleted_at": now,
		"revision":   gorm.Expr("revision + 1"),
	}).Error
}

func (r *sqliteRepository) SoftDeleteSqlTx(tx *sql.Tx, id ID) error {
	now := time.Now().Unix()
	_, err := tx.Exec(
		`UPDATE artifacts SET status = ?, deleted_at = datetime(?, 'unixepoch'), revision = revision + 1 WHERE artifact_id = ?`,
		StatusDeleted, now, id,
	)
	return err
}

func (r *sqliteRepository) InsertReference(tx *gorm.DB, ref *ArtifactReference) error {
	return tx.Create(ref).Error
}

func (r *sqliteRepository) InsertReferenceSqlTx(tx *sql.Tx, ref *ArtifactReference) error {
	_, err := tx.Exec(
		`INSERT INTO artifact_references (artifact_id, reference_type, reference_id, created_at) VALUES (?, ?, ?, ?)`,
		ref.ArtifactID, ref.ReferenceType, ref.ReferenceID, ref.CreatedAt,
	)
	return err
}

func (r *sqliteRepository) RemoveReferenceSqlTx(tx *sql.Tx, artifactID ID, refType string, refID string) error {
	_, err := tx.Exec(
		`DELETE FROM artifact_references WHERE artifact_id = ? AND reference_type = ? AND reference_id = ?`,
		artifactID, refType, refID,
	)
	return err
}

func (r *sqliteRepository) CountReferences(artifactID ID) (int64, error) {
	var count int64
	err := r.db.Model(&ArtifactReference{}).Where("artifact_id = ?", artifactID).Count(&count).Error
	return count, err
}

func (r *sqliteRepository) CountReferencesSqlTx(tx *sql.Tx, artifactID ID) (int64, error) {
	var count int64
	err := tx.QueryRow(
		`SELECT COUNT(*) FROM artifact_references WHERE artifact_id = ?`,
		artifactID,
	).Scan(&count)
	return count, err
}
