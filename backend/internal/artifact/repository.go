package artifact

import (
	"gorm.io/gorm"
)

type Repository interface {
	Create(tx *gorm.DB, artifact *Artifact) error
	GetByID(id ID) (*Artifact, error)
	GetByOwnerAndID(ownerUserID string, id ID) (*Artifact, error)
	SoftDelete(tx *gorm.DB, id ID) error
	InsertReference(tx *gorm.DB, ref *ArtifactReference) error
	CountReferences(artifactID ID) (int64, error)
	DB() *gorm.DB
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

func (r *sqliteRepository) Create(tx *gorm.DB, artifact *Artifact) error {
	return tx.Create(artifact).Error
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

func (r *sqliteRepository) InsertReference(tx *gorm.DB, ref *ArtifactReference) error {
	return tx.Create(ref).Error
}

func (r *sqliteRepository) CountReferences(artifactID ID) (int64, error) {
	var count int64
	err := r.db.Model(&ArtifactReference{}).Where("artifact_id = ?", artifactID).Count(&count).Error
	return count, err
}
